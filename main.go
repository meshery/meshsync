package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/layer5io/meshkit/broker/nats"
	configprovider "github.com/layer5io/meshkit/config/provider"
	"github.com/layer5io/meshkit/logger"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	"github.com/layer5io/meshsync/internal/channels"
	"github.com/layer5io/meshsync/internal/config"
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
	viper.SetDefault("BUILD", version)
	viper.SetDefault("COMMITSHA", commitsha)

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

	// get configs from meshsync crd if available
	crdConfigs, err := config.GetMeshsyncCRDConfigs(kubeClient.DynamicKubeClient)
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

	err = config.PatchCRVersion(&kubeClient.RestConfig)
	if err != nil {
		log.Warn(err)
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
	//Skip/Comment the below connectivity test in local environment
	connectivityTest(cfg.GetKey(config.BrokerURL), log)
	// Initialize Broker instance
	br, err := nats.New(nats.Options{
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

	chPool := channels.NewChannelPool()
	meshsyncHandler, err := meshsync.New(cfg, log, br, chPool)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	go meshsyncHandler.WatchCRDs()

	go meshsyncHandler.Run()
	go meshsyncHandler.ListenToRequests()

	log.Info("Server started")
	// Handle graceful shutdown
	signal.Notify(chPool[channels.OS].(channels.OSChannel), syscall.SIGTERM, os.Interrupt)
	select {
	case <-chPool[channels.OS].(channels.OSChannel):
		close(chPool[channels.Stop].(channels.StopChannel))
		log.Info("Shutting down")
	case <-chPool[channels.Stop].(channels.StopChannel):
		close(chPool[channels.Stop].(channels.StopChannel))
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
