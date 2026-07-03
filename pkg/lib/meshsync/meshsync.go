// this package contains what previously was run in main.go arranged as a library
package meshsync

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/broker/nats"
	"github.com/meshery/meshkit/logger"
	mesherykube "github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/meshsync/internal/channels"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/internal/file"
	"github.com/meshery/meshsync/internal/output"
	"github.com/meshery/meshsync/meshsync"
)

// TODO fix cyclop error
// Error: main.go:46:1: calculated cyclomatic complexity for function mainWithExitCode is 25, max is 10 (cyclop)
//
//nolint:cyclop
func Run(log logger.Handler, optsSetters ...OptionsSetter) error {
	options := DefautOptions
	for _, setOptions := range optsSetters {
		// test case: "output mode channel: must not fail when has nil in options setter"
		if setOptions != nil {
			setOptions(&options)
		}
	}
	if !slices.Contains(AllowedOutputModes, options.OutputMode) {
		return fmt.Errorf(
			"unsupported output mode \"%s\", supported list is [%s]",
			options.OutputMode,
			strings.Join(AllowedOutputModes, ", "),
		)
	}

	// Serve liveness/readiness endpoints as early as possible so that
	// /healthz responds even while the broker is still unreachable; shut them
	// down when Run exits so library callers don't leak the port/goroutine.
	health := newHealthServer()
	stopHealth := health.start(log, ":"+config.Server["port"])
	defer stopHealth()

	// Initialize kubeclient
	// options.KubeConfig is nil by default
	kubeClient, err := mesherykube.New(options.KubeConfig)
	if err != nil {
		return err
	}

	useCRDFlag := determineUseCRDFlag(options, log, kubeClient)

	crdConfigs, errGetMeshsyncCRDConfigs := getMeshsyncCRDConfigs(useCRDFlag, kubeClient)
	if errGetMeshsyncCRDConfigs != nil {
		// no configs found from meshsync CRD log warning
		log.Warnf("meshsync: %v", errGetMeshsyncCRDConfigs)
	}
	// Config init and seed
	cfg, err := config.New(options.MeshkitConfigProvider)
	if err != nil {
		return err
	}

	config.Server["version"] = options.Version
	err = cfg.SetObject(config.ServerKey, config.Server)
	if err != nil {
		return err
	}

	// MeshSync no longer writes its build version into its own CR: spec is the
	// operator's (and the user's) declaration of DESIRED state — the operator
	// maps spec.version to the image tag — so a self-report there fights the
	// controller and can roll the Deployment to the reporter's own version.
	// The running version is still advertised over the broker (meshsync-meta).

	// pass configs from crd to default configs
	if crdConfigs != nil {
		// Assign the CRD-derived pipelines whenever a config was produced, even
		// if the watch-list filtered down to zero pipelines: an explicit
		// whitelist/blacklist that matches nothing means "watch nothing" and must
		// not silently fall back to the full default set. The default pipelines
		// apply only when there was no CRD config at all (crdConfigs == nil), e.g.
		// a CRD read/parse error; in that case config.Pipelines keeps its package
		// default, whose Events are backfilled in config's init() so it still
		// publishes rather than silently dropping everything.
		config.Pipelines = crdConfigs.Pipelines

		if len(crdConfigs.Listeners) > 0 {
			config.Listeners = crdConfigs.Listeners
		}
	}

	cfg.SetKey(config.BrokerURL, os.Getenv("BROKER_URL"))

	err = cfg.SetObject(config.ResourcesKey, config.Pipelines)
	if err != nil {
		return err
	}

	err = cfg.SetObject(config.ListenersKey, config.Listeners)
	if err != nil {
		return err
	}

	outputProcessor := output.NewProcessor()
	var br broker.Handler
	if options.OutputMode == config.OutputModeBroker {
		// take from options; if nil, instantiate;
		// this allows to provide custom implementation of broker.Handler interface
		br = options.BrokerHandler
		if br == nil {
			brokerHandler, errNatsNew := createNatsBrokerHandler(
				log,
				options.PingEndpoint,
				cfg.GetKey(config.BrokerURL),
			)
			if errNatsNew != nil {
				return errNatsNew
			}
			br = brokerHandler
		}
		// the broker handler exists (nats.New succeeded or a custom
		// handler was provided): report ready on /readyz
		health.markReady()
		outputProcessor.SetOutput(
			output.NewBrokerWriter(
				br,
			),
		)
	}

	if options.OutputMode == config.OutputModeFile {
		filename := options.OutputFileName
		defaultFormat := "yaml"
		if filename == "" {
			fname, errGenerateUniqueFileNameForSnapshot := file.GenerateUniqueFileNameForSnapshot(defaultFormat)
			if errGenerateUniqueFileNameForSnapshot != nil {
				return errGenerateUniqueFileNameForSnapshot
			}
			filename = fname
		}
		ext := path.Ext(filename)
		if ext == "" {
			ext = "." + defaultFormat
		}

		writerPool := make([]output.Writer, 0, 2)

		if options.OutputExtendedFile {
			// this is a file which contains all messages from nats
			// (hence it also contains more than one yaml manifest for the same entity)
			fw, errNewYAMLWriter := file.NewYAMLWriter(
				fmt.Sprintf(
					"%s-extended%s",
					strings.TrimSuffix(filename, ext),
					ext,
				),
			)
			if errNewYAMLWriter != nil {
				return errNewYAMLWriter
			}
			// if you do refactoring and move this in a separate function
			// be sure this Close is called in the end of Run() function
			defer fw.Close()

			writerPool = append(
				writerPool,
				output.NewFileWriter(fw),
			)
		}

		{
			// this is a file which contains only unique resource's messages from nats
			// it filters out duplicates and writes only unique message from nats per resource
			fw, errNewYAMLWriter := file.NewYAMLWriter(filename)
			if errNewYAMLWriter != nil {
				return errNewYAMLWriter
			}
			outputInMemoryDeduplicatorWriter := output.NewInMemoryDeduplicatorStreamingWriter(
				output.NewFileWriter(fw),
			)

			writerPool = append(writerPool, outputInMemoryDeduplicatorWriter)
		}

		outputProcessor.SetOutput(
			output.NewCompositeWriter(
				writerPool...,
			),
		)
	}

	chPool := channels.NewChannelPool()
	meshsyncHandler, err := meshsync.New(
		cfg,
		kubeClient,
		log,
		br,
		outputProcessor,
		chPool,
		config.NewOutputFiltrationContainer(
			config.NewOutputNamespaceSet(options.OnlyK8sNamespaces...),
			config.NewOutputResourceSet(options.OnlyK8sResources),
		),
	)
	if err != nil {
		return err
	}
	defer meshsyncHandler.ShutdownInformer()

	if useCRDFlag {
		go meshsyncHandler.WatchCRDs()
	}

	// Start the main meshsync run
	go meshsyncHandler.Run()

	if options.OutputMode == config.OutputModeBroker {
		// even so the config param name starts with OutputMode
		// it is not only output but also input
		// in that case if  OutputMode is not OutputModeBroker
		// there is no nats at all, so we do not subscribe to any topic
		go meshsyncHandler.ListenToRequests()
	}

	chTimeout := make(chan struct{})
	if options.StopAfterDuration > -1 {
		go func(ch chan struct{}) {
			<-time.After(options.StopAfterDuration)
			log.Debugf("meshsync: stopping after %s", options.StopAfterDuration)
			close(chTimeout)
		}(chTimeout)
	}

	log.Info("meshsync: run started")
	// Handle graceful shutdown
	signal.Notify(chPool[channels.OS].(channels.OSChannel), syscall.SIGTERM, os.Interrupt)

	select {
	case <-chTimeout:
	case <-chPool[channels.OS].(channels.OSChannel):
	case <-options.Context.Done():
		log.Debug("meshsync: cancellation signal received from client code")
	}

	// close stop channel
	// as there are many goroutines which wait for channels.Stop to be closed to stop their execution
	close(chPool[channels.Stop].(channels.StopChannel))

	log.Info("meshsync: shutting down")

	return nil
}

// connectivityTestTimeout bounds how long connectivityTest keeps retrying
// before it gives up and returns an error.
const connectivityTestTimeout = 5 * time.Minute

// brokerHost extracts the host part of a broker URL, stripping the scheme,
// credentials (userinfo) and port if present. URLs without a scheme, such as
// "host:4222", are treated as nats:// URLs. IPv6 literals are returned
// without brackets.
func brokerHost(brokerURL string) (string, error) {
	rawURL := brokerURL
	if !strings.Contains(rawURL, "://") {
		rawURL = "nats://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid broker url %q: %w", brokerURL, err)
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("no host found in broker url %q", brokerURL)
	}
	return host, nil
}

// connectivityTest makes sure the broker monitoring endpoint is reachable
// before the NATS client is started. It retries once per second and gives up
// once the timeout elapses, returning an error so that the caller exits
// visibly instead of spinning forever.
func connectivityTest(log logger.Handler, pingEndpoint string, brokerURL string, timeout time.Duration) error {
	host, err := brokerHost(brokerURL)
	if err != nil {
		return err
	}
	pingURL := "http://" + host + pingEndpoint
	deadline := time.Now().Add(timeout)
	for {
		errPing := pingBroker(pingURL)
		if errPing == nil {
			return nil
		}
		log.Warnf("meshsync: broker connectivity test failed for %s: %v, retrying...", pingURL, errPing)
		if time.Now().After(deadline) {
			return fmt.Errorf("broker connectivity test did not succeed within %s: %s: %w", timeout, pingURL, errPing)
		}
		time.Sleep(1 * time.Second)
	}
}

// pingBroker performs a single HTTP GET against the broker monitoring
// endpoint and reports any failure to reach it or non-OK response status.
// The client carries its own timeout: http.DefaultClient has none, and a
// black-holed endpoint would otherwise hang the request past the overall
// connectivityTest deadline.
func pingBroker(pingURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(pingURL) //nolint
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status %q", resp.Status)
	}
	return nil
}

func createNatsBrokerHandler(log logger.Handler, pingEndpoint string, brokerURL string) (broker.Handler, error) {
	if err := connectivityTest(
		log,
		pingEndpoint,
		brokerURL,
		connectivityTestTimeout,
	); err != nil {
		return nil, err
	}
	return nats.New(nats.Options{
		URLS:           []string{brokerURL},
		ConnectionName: "meshsync",
		Username:       "",
		Password:       "",
		ReconnectWait:  2 * time.Second,
		MaxReconnect:   60,
	})
}

func determineUseCRDFlag(
	options Options,
	log logger.Handler,
	kubeClient *mesherykube.Client,
) bool {
	// if output mode is not nats generally it is not expected to have CRD present in cluster.
	// theoretically CRD could be present even in file, channel output mode.
	// hence check if CRD are present in the cluster,
	// and only skip them if it is not present.
	crd, errGetMeshsyncCRD := config.GetMeshsyncCRD(kubeClient.DynamicKubeClient)
	useCRDFlag := crd != nil && errGetMeshsyncCRD == nil
	if useCRDFlag {
		log.Debugf(
			"meshsync: running in %s output mode and meshsync CRD is present in the cluster",
			options.OutputMode,
		)
	} else {
		log.Debugf(
			"meshsync: running in %s mode and NO meshsync CRD is present in the cluster",
			options.OutputMode,
		)
	}

	return useCRDFlag
}

func getMeshsyncCRDConfigs(useCRDFlag bool, kubeClient *mesherykube.Client) (*config.MeshsyncConfig, error) {
	if useCRDFlag {
		// get configs from meshsync crd if available
		return config.GetMeshsyncCRDConfigs(kubeClient.DynamicKubeClient)
	}
	// get configs from local variable
	return config.GetMeshsyncCRDConfigsLocal()
}
