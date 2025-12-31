// this package contains what previously was run in main.go arranged as a library
package meshsync

import (
	"errors"
	"fmt"
	"net/http"
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

	if useCRDFlag {
		// Only attempt to patch MeshSync CR/version if the MeshSync CR object exists.
		if _, errGetCR := config.GetMeshsyncCRD(kubeClient.DynamicKubeClient); errGetCR == nil {
			if errPatchCRVersion := config.PatchCRVersion(&kubeClient.RestConfig); errPatchCRVersion != nil {
				log.Warnf("meshsync: %v", errPatchCRVersion)
			}
		} else {
			log.Debugf("meshsync: skipping PatchCRVersion because MeshSync CR not found: %v", errGetCR)
		}
	}

	// pass configs from crd to default configs
	if crdConfigs != nil {
		if len(crdConfigs.Pipelines) > 0 {
			config.Pipelines = crdConfigs.Pipelines
		}

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

func connectivityTest(log logger.Handler, pingEndpoint string, url string) error {
	// Make sure Broker has started before starting NATS client
	urls := strings.Split(url, ":")
	if len(urls) == 0 {
		return errors.New("invalid URL")
	}
	pingURL := "http://" + urls[0] + pingEndpoint
	for {
		resp, err := http.Get(pingURL) //nolint
		if err != nil {
			log.Debugf("meshsync: could not connect to broker: %v retrying...", err)
			time.Sleep(1 * time.Second)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		log.Debugf("meshsync: could not receive OK response from broker: %s retrying...", pingURL)
		time.Sleep(1 * time.Second)
	}

	return nil
}

func createNatsBrokerHandler(log logger.Handler, pingEndpoint string, brokerURL string) (broker.Handler, error) {
	if err := connectivityTest(
		log,
		pingEndpoint,
		brokerURL,
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
