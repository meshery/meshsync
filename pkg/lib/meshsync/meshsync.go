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

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/broker/nats"
	"github.com/layer5io/meshkit/logger"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	"github.com/layer5io/meshsync/internal/channels"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/file"
	"github.com/layer5io/meshsync/internal/output"
	"github.com/layer5io/meshsync/meshsync"
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
	kubeClient, err := mesherykube.New(nil)
	if err != nil {
		return err
	}

	useCRDFlag := determineUseCRDFlag(options, log, kubeClient)

	crdConfigs, errGetMeshsyncCRDConfigs := getMeshsyncCRDConfigs(useCRDFlag, kubeClient)
	if errGetMeshsyncCRDConfigs != nil {
		// no configs found from meshsync CRD log warning
		log.Warn(err)
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
		// this patch only make sense when CRD is present in cluster
		if errPatchCRVersion := config.PatchCRVersion(&kubeClient.RestConfig); errPatchCRVersion != nil {
			log.Warn(errPatchCRVersion)
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
	if options.OutputMode == config.OutputModeNats {
		// Skip/Comment the below connectivity test in local environment
		if errConnectivityTest := connectivityTest(
			log,
			options.PingEndpoint,
			cfg.GetKey(config.BrokerURL),
		); errConnectivityTest != nil {
			return errConnectivityTest
		}
		// Initialize Broker instance
		broker, errNatsNew := nats.New(nats.Options{
			URLS:           []string{cfg.GetKey(config.BrokerURL)},
			ConnectionName: "meshsync",
			Username:       "",
			Password:       "",
			ReconnectWait:  2 * time.Second,
			MaxReconnect:   60,
		})
		if errNatsNew != nil {
			return errNatsNew
		}
		br = broker
		outputProcessor.SetOutput(
			output.NewNatsWriter(
				br,
			),
		)
	}

	if options.OutputMode == config.OutputModeFile {
		filename := config.OutputFileName
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
		defer fw.Close()

		// this is a file which contains only unique resource's messages from nats
		// it filters out duplicates and writes only latest message from nats per resource
		fw2, errNewYAMLWriter2 := file.NewYAMLWriter(filename)
		if errNewYAMLWriter2 != nil {
			return errNewYAMLWriter2
		}
		// this one not written immediately,
		// but collects in memory and flushes in the end
		outputInMemoryDeduplicatorWriter := output.NewInMemoryDeduplicatorWriter(
			output.NewFileWriter(fw2),
		)
		// ensure to flush
		defer outputInMemoryDeduplicatorWriter.Flush()

		outputProcessor.SetOutput(
			output.NewCompositeWriter(
				output.NewFileWriter(fw),
				outputInMemoryDeduplicatorWriter,
			),
		)
	}

	if options.OutputMode == config.OutputModeChannel {
		if options.TransportChannel == nil {
			return errors.New("options.transportChannel is nil")
		}
		outputProcessor.SetOutput(
			output.NewChannelWriter(options.TransportChannel),
		)
	}

	chPool := channels.NewChannelPool()
	meshsyncHandler, err := meshsync.New(cfg, log, br, outputProcessor, chPool)
	if err != nil {
		return err
	}

	go meshsyncHandler.WatchCRDs()

	go meshsyncHandler.Run()
	// TODO
	// as we have introduced a new output mode channel
	// do we need to have a ListenToRequests channel?
	if options.OutputMode == config.OutputModeNats {
		// even so the config param name is OutputMode
		// it is not only output but also input
		// in that case if  OutputMode is not OutputModeNats
		// there is no nats at all, so we do not subscribe to any topic
		go meshsyncHandler.ListenToRequests()
	}

	if options.StopAfterDuration > -1 {
		go func(stopCh channels.StopChannel) {
			<-time.After(options.StopAfterDuration)
			log.Infof("Stopping after %s", options.StopAfterDuration)
			stopCh <- struct{}{}
			// close(stopCh)
		}(chPool[channels.Stop].(channels.StopChannel))
	}

	log.Info("Server started")
	// Handle graceful shutdown
	signal.Notify(chPool[channels.OS].(channels.OSChannel), syscall.SIGTERM, os.Interrupt)
	select {
	case <-chPool[channels.OS].(channels.OSChannel):
		close(chPool[channels.Stop].(channels.StopChannel))
	case <-chPool[channels.Stop].(channels.StopChannel):
		// // NOTE:
		// // does not make sense to close the StopChannel here,
		// // as the general approach with stop channel to close it rather then put smth in it,
		// // and hence next close will create panic if stop channel is already closed
		// // so commented this out:
		// close(chPool[channels.Stop].(channels.StopChannel))
	}

	log.Info("Shutting down")

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
			log.Info("could not connect to broker: " + err.Error() + " retrying...")
			time.Sleep(1 * time.Second)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		log.Info("could not receive OK response from broker: "+pingURL, " retrying...")
		time.Sleep(1 * time.Second)
	}

	return nil
}

func determineUseCRDFlag(
	options Options,
	log logger.Handler,
	kubeClient *mesherykube.Client,
) bool {
	useCRDFlag := true
	if options.OutputMode == config.OutputModeFile {
		// if output mode is file -> generally it is not expected to have CRD present in cluster.
		// theoretically CRDs could be present even in file output mode.
		// hence check if CRD is present in the cluster,
		// and only skip them in file output mode if it is not present.
		crd, errGetMeshsyncCRD := config.GetMeshsyncCRD(kubeClient.DynamicKubeClient)
		if crd != nil && errGetMeshsyncCRD == nil {
			// this is rare, but valid case
			log.Info("running in file output mode and meshsync CRD is present in the cluster")
		} else {
			useCRDFlag = false
			// this is the most common case, file mode and no CRD
			log.Info("running in file output mode and NO meshsync CRD is present in the cluster (expected behaviour)")
		}
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
