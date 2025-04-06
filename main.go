package main

import (
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

func init() {
	// this function is also executed in tests
	// having flag.Parse() leads to an error
	// flag provided but not defined: -test.testlogfile
	// because go defined custom flags during tests run
	// moved flags to parseFlags() and call in main()

}

func main() {
	parseFlags()
	viper.SetDefault("BUILD", version)
	viper.SetDefault("COMMITSHA", commitsha)

	// if output mode is file -> do not try to use meshsync CRD.
	// TODO
	// theoretically CRDs could be present even in file output mode
	// circle around the opportunity to check if CRD is present in the cluster,
	// and only skip them in file output mode if it is not present.
	skipCRDFlag := config.OutputMode == config.OutputModeFile

	// Initialize Logger instance
	log, err := logger.New(serviceName, logger.Options{
		Format:   logger.SyslogLogFormat,
		LogLevel: int(logrus.InfoLevel),
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Initialize kubeclient
	kubeClient, err := mesherykube.New(nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var crdConfigs *config.MeshsyncConfig

	if skipCRDFlag {
		// get configs from local variable
		crdConfigs, err = config.GetMeshsyncCRDConfigsLocal()
	} else {
		// get configs from meshsync crd if available
		crdConfigs, err = config.GetMeshsyncCRDConfigs(kubeClient.DynamicKubeClient)
	}
	if err != nil {
		// no configs found from meshsync CRD log warning
		log.Warn(err)
	}
	// Config init and seed
	cfg, err := config.New(provider)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	config.Server["version"] = version
	err = cfg.SetObject(config.ServerKey, config.Server)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	if !skipCRDFlag {
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
		os.Exit(1)
	}

	err = cfg.SetObject(config.ListenersKey, config.Listeners)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	outputProcessor := output.NewProcessor()
	var br broker.Handler
	if config.OutputMode == config.OutputModeNats {
		//Skip/Comment the below connectivity test in local environment
		connectivityTest(cfg.GetKey(config.BrokerURL), log)
		// Initialize Broker instance
		broker, err := nats.New(nats.Options{
			URLS:           []string{cfg.GetKey(config.BrokerURL)},
			ConnectionName: "meshsync",
			Username:       "",
			Password:       "",
			ReconnectWait:  2 * time.Second,
			MaxReconnect:   60,
		})
		if err != nil {
			log.Error(err)
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
			fname, err := file.GenerateUniqueFileNameForSnapshot("yaml")
			if err != nil {

			}
			filename = fname
		}
		fw, err := file.NewYAMLWriter(filename)
		if err != nil {
			fmt.Println(err)
			return
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
		os.Exit(1)
	}

	go meshsyncHandler.WatchCRDs()

	go meshsyncHandler.Run()
	go meshsyncHandler.ListenToRequests()

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
}

func connectivityTest(url string, log logger.Handler) {
	// Make sure Broker has started before starting NATS client
	urls := strings.Split(url, ":")
	if len(urls) == 0 {
		log.Info("invalid URL")
		os.Exit(1)
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
