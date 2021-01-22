package main

import (
	"fmt"
	"os"

	configprovider "github.com/layer5io/meshery-adapter-library/config/provider"
	"github.com/layer5io/meshkit/logger"
	service "github.com/layer5io/meshsync/api/grpc"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/meshsync"
	"github.com/layer5io/meshsync/pkg/broker/nats"
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

	cfg, err := config.New(provider)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Initialize Broker instance
	br, err := nats.New(cfg.GetKey(config.BrokerURL))
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	meshsyncHandler, err := meshsync.New(cfg, log, br)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	err = meshsyncHandler.StartDiscovery()
	if err != nil {
		log.Error(err)
	}

	// Initialize service by running pre-defined tasks
	sHandler := &service.Service{
		Handler: meshsyncHandler,
	}
	err = cfg.GetObject(config.ServerConfig, &sHandler)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Start GRPC server
	log.Info("Controller Listening at port: ", sHandler.Port)
	err = service.Start(sHandler)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
