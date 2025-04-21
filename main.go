// TODO fix cyclop error
// Error: main.go:1:1: the average complexity for the package main is 8.000000, max is 7.000000 (cyclop)
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

func init() {}

func main() {
	if exitCode := mainWithExitCode(); exitCode != 0 {
		os.Exit(exitCode)
	}
}

// TODO fix cyclop error
// main.go:51:1: calculated cyclomatic complexity for function mainWithExitCode is 29, max is 10 (cyclop)
//
//nolint:cyclop
func mainWithExitCode() int {
	parseFlags()
	viper.SetDefault("BUILD", version)
	viper.SetDefault("COMMITSHA", commitsha)

	// Initialize Logger instance
	log, err := logger.New(serviceName, logger.Options{
		Format:   logger.SyslogLogFormat,
		LogLevel: int(logrus.InfoLevel),
	})
	if err != nil {
		fmt.Println(err)
		return 1
	}

	// Initialize kubeclient
	kubeClient, err := mesherykube.New(nil)
	if err != nil {
		fmt.Println(err)
		return 1
	}

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

	var crdConfigs *config.MeshsyncConfig

	if useCRDFlag {
		// get configs from meshsync crd if available
		crdConfigs, err = config.GetMeshsyncCRDConfigs(kubeClient.DynamicKubeClient)
	} else {
		// get configs from local variable
		crdConfigs, err = config.GetMeshsyncCRDConfigsLocal()

	}
	if err != nil {
		// no configs found from meshsync CRD log warning
		log.Warn(err)
	}
	// Config init and seed
	cfg, err := config.New(provider)
	if err != nil {
		log.Error(err)
		return 1
	}

	config.Server["version"] = version
	err = cfg.SetObject(config.ServerKey, config.Server)
	if err != nil {
		log.Error(err)
		return 1
	}

	if useCRDFlag {
		// this patch only make sense when CRD is present in cluster
		err = config.PatchCRVersion(&kubeClient.RestConfig)
		if err != nil {
			log.Warn(err)
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
		log.Error(err)
		return 1
	}

	err = cfg.SetObject(config.ListenersKey, config.Listeners)
	if err != nil {
		log.Error(err)
		return 1
	}

	outputProcessor := output.NewProcessor()
	var br broker.Handler
	if config.OutputMode == config.OutputModeNats {
		// Skip/Comment the below connectivity test in local environment
		if exitCode := connectivityTest(cfg.GetKey(config.BrokerURL), log); exitCode != 0 {
			return exitCode
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
			log.Error(errNatsNew)
			os.Exit(1)
		}
		br = broker
		outputProcessor.SetStrategy(
			output.NewNatsStrategy(
				br,
			),
		)
	}

	if config.OutputMode == config.OutputModeFile {
		filename := config.OutputFileName
		if filename == "" {
			fname, errGenerateUniqueFileNameForSnapshot := file.GenerateUniqueFileNameForSnapshot("yaml")
			if errGenerateUniqueFileNameForSnapshot != nil {
				log.Error(errGenerateUniqueFileNameForSnapshot)
				os.Exit(1)
			}
			filename = fname
		}
		fw, errNewYAMLWriter := file.NewYAMLWriter(filename)
		if errNewYAMLWriter != nil {
			fmt.Println(errNewYAMLWriter)
			return 1
		}
		defer fw.Close()
		outputProcessor.SetStrategy(
			output.NewFileStrategy(
				fw,
			),
		)
	}

	chPool := channels.NewChannelPool()
	meshsyncHandler, err := meshsync.New(cfg, log, br, outputProcessor, chPool)
	if err != nil {
		log.Error(err)
		return 1
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
		log.Info("Shutting down")
	case <-chPool[channels.Stop].(channels.StopChannel):
		// // NOTE:
		// // does not make sense to close the StopChannel here,
		// // as the general approach with stop channel to close it rather then put smth in it,
		// // and hence next close will create panic if stop channel is already closed
		// // so commented this out:
		// close(chPool[channels.Stop].(channels.StopChannel))
		log.Info("Shutting down")
	}

	return 0
}

func connectivityTest(url string, log logger.Handler) int {
	// Make sure Broker has started before starting NATS client
	urls := strings.Split(url, ":")
	if len(urls) == 0 {
		log.Error(errors.New("invalid URL"))
		return 1
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

	return 0
}

func parseFlags() {
	flag.StringVar(
		&config.OutputMode,
		"output",
		config.OutputModeNats,
		fmt.Sprintf("Output mode: '%s' or '%s'", config.OutputModeNats, config.OutputModeFile),
	)
	flag.StringVar(
		&config.OutputFileName,
		"outputFile",
		"",
		"Output file path (default: meshery-cluster-snapshot-YYYYMMDD-00.yaml in the current directory)",
	)
	flag.StringVar(
		&config.OutputNamespace,
		"outputNamespace",
		"",
		"namespace for which limit output to file",
	)
	var outputResourcesString string
	flag.StringVar(
		&outputResourcesString,
		"outputResources",
		"",
		"resources for which limit output to file, coma separated list of k8s resources, f.e. pod,deployment,service",
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
