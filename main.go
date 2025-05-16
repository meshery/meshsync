// TODO fix cyclop error
// Error: main.go:1:1: the average complexity for the package main is 7.166667, max is 7.000000 (cyclop)
//
//nolint:cyclop
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/broker/nats"
	configprovider "github.com/layer5io/meshkit/config/provider"
	"github.com/layer5io/meshkit/logger"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	"github.com/layer5io/meshsync/internal/channels"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/file"
	"github.com/layer5io/meshsync/internal/output"
	"github.com/layer5io/meshsync/meshsync"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	serviceName  = "meshsync"
	provider     = configprovider.ViperKey
	version      = "Not Set"
	commitsha    = "Not Set"
	pingEndpoint = ":8222/connz"
)

func main() {
	parseFlags()
	viper.SetDefault("BUILD", version)
	viper.SetDefault("COMMITSHA", commitsha)

	// Initialize Logger instance
	log, errLoggerNew := logger.New(serviceName, logger.Options{
		Format:   logger.SyslogLogFormat,
		LogLevel: int(logrus.InfoLevel),
	})
	if errLoggerNew != nil {
		fmt.Println(errLoggerNew)
		os.Exit(1)
	}

	if err := mainWithError(log); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

// TODO fix cyclop error
// Error: main.go:46:1: calculated cyclomatic complexity for function mainWithExitCode is 25, max is 10 (cyclop)
//
//nolint:cyclop
func mainWithError(log logger.Handler) error {
	// Initialize kubeclient
	kubeClient, err := mesherykube.New(nil)
	if err != nil {
		return err
	}

	useCRDFlag := determineUseCRDFlag(log, kubeClient)

	crdConfigs, errGetMeshsyncCRDConfigs := getMeshsyncCRDConfigs(useCRDFlag, kubeClient)
	if errGetMeshsyncCRDConfigs != nil {
		// no configs found from meshsync CRD log warning
		log.Warn(err)
	}
	// Config init and seed
	cfg, err := config.New(provider)
	if err != nil {
		return err
	}

	config.Server["version"] = version
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
	if config.OutputMode == config.OutputModeNats {
		// Skip/Comment the below connectivity test in local environment
		if errConnectivityTest := connectivityTest(cfg.GetKey(config.BrokerURL), log); errConnectivityTest != nil {
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

	if config.OutputMode == config.OutputModeFile {
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

	chPool := channels.NewChannelPool()
	meshsyncHandler, err := meshsync.New(cfg, log, br, outputProcessor, chPool)
	if err != nil {
		return err
	}

	go meshsyncHandler.WatchCRDs()

	go meshsyncHandler.Run()
	if config.OutputMode == config.OutputModeNats {
		// even so the config param name is OutputMode
		// it is not only output but also input
		// in that case if  OutputMode is not OutputModeNats
		// there is no nats at all, so we do not subscribe to any topic
		go meshsyncHandler.ListenToRequests()
	}

	if config.StopAfterSeconds > -1 {
		go func(stopCh channels.StopChannel) {
			<-time.After(time.Second * time.Duration(config.StopAfterSeconds))
			log.Infof("Stopping after %d seconds", config.StopAfterSeconds)
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

func connectivityTest(url string, log logger.Handler) error {
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

func parseFlags() {
	flag.StringVar(
		&config.OutputMode,
		"output",
		config.OutputModeNats,
		fmt.Sprintf("output mode: \"%s\" or \"%s\"", config.OutputModeNats, config.OutputModeFile),
	)
	flag.StringVar(
		&config.OutputFileName,
		"outputFile",
		"",
		"output file where to put the meshsync events (cluster snapshot), only applicable for file output mode (default \"./meshery-cluster-snapshot-YYYYMMDD-00.yaml\")",
	)
	flag.StringVar(
		&config.OutputNamespace,
		"outputNamespace",
		"",
		"k8s namespace for which limit the output, f.e. \"default\", applicable for both nats and file output mode",
	)
	var outputResourcesString string
	flag.StringVar(
		&outputResourcesString,
		"outputResources",
		"",
		"k8s resources for which limit the output, coma separated case insensitive list of k8s resources, f.e. \"pod,deployment,service\", applicable for both nats and file output mode",
	)
	flag.IntVar(
		&config.StopAfterSeconds,
		"stopAfterSeconds",
		-1,
		"stop meshsync execution after specified amount of seconds",
	)

	// Parse the command=line flags to get the output mode
	flag.Parse()

	config.OutputResourcesSet = make(map[string]bool)
	if outputResourcesString != "" {
		config.OutputOnlySpecifiedResources = true
		outputResourcesList := strings.Split(outputResourcesString, ",")
		for _, item := range outputResourcesList {
			config.OutputResourcesSet[strings.ToLower(item)] = true
		}
	}
}

func determineUseCRDFlag(log logger.Handler, kubeClient *mesherykube.Client) bool {
	useCRDFlag := true
	if config.OutputMode == config.OutputModeFile {
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
