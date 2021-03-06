package main

import (
	"fmt"
	"os"
	"time"

	"github.com/layer5io/meshkit/broker/nats"
	configprovider "github.com/layer5io/meshkit/config/provider"
	"github.com/layer5io/meshkit/logger"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/meshsync"
)

var (
	serviceName = "meshsync"
	provider    = configprovider.ViperKey
)

func main() {
	// Initialize Logger instance
	log, err := logger.New(serviceName, logger.Options{
		Format: logger.SyslogLogFormat,
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Config init and seed
	cfg, err := config.New(provider)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	cfg.SetKey(config.BrokerURL, os.Getenv("BROKER_URL"))
	err = cfg.SetObject(config.ServerKey, config.Server)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	err = cfg.SetObject(config.ResourcesKey, config.Pipelines)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	// Seeding done

	// Initialize Broker instance
	br, err := nats.New(nats.Options{
		URLS:           []string{cfg.GetKey(config.BrokerURL)},
		ConnectionName: "meshsync",
		Username:       "",
		Password:       "",
		ReconnectWait:  2 * time.Second,
		MaxReconnect:   5,
	})
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	meshsyncHandler, err := meshsync.New(cfg, log, br)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	err = meshsyncHandler.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
