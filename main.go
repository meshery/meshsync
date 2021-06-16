package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/layer5io/meshkit/broker/nats"
	configprovider "github.com/layer5io/meshkit/config/provider"
	"github.com/layer5io/meshkit/logger"
	"github.com/layer5io/meshsync/internal/channels"
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

	err = cfg.SetObject(config.ListenersKey, config.Listeners)
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

	chPool := channels.NewChannelPool()
	meshsyncHandler, err := meshsync.New(cfg, log, br, chPool)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

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
