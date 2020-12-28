package config

import (
	"os"
	"time"

	"github.com/layer5io/meshery-adapter-library/config"
	configprovider "github.com/layer5io/meshery-adapter-library/config/provider"
	"github.com/layer5io/meshkit/utils"
)

var (
	server = map[string]string{
		"name":      "meshery-meshsync",
		"port":      "11000",
		"version":   "v0.0.1-alpha3",
		"startedat": time.Now().String(),
	}
)

// New creates a new config instance
func New(provider string) (config.Handler, error) {

	var (
		handler config.Handler
		err     error
	)

	opts := configprovider.Options{
		ProviderConfig: map[string]string{
			configprovider.FilePath: utils.GetHome(),
			configprovider.FileType: "yaml",
			configprovider.FileName: "config",
		},
	}

	// Config provider
	switch provider {
	case configprovider.ViperKey:
		handler, err = configprovider.NewViper(opts)
		if err != nil {
			return nil, ErrInitConfig(err)
		}
	case configprovider.InMemKey:
		handler, err = configprovider.NewInMem(opts)
		if err != nil {
			return nil, ErrInitConfig(err)
		}
	}

	err = initConfig(handler)
	if err != nil {
		return nil, ErrInitConfig(err)
	}

	return handler, nil
}

func initConfig(cfg config.Handler) error {
	cfg.SetKey(BrokerURL, os.Getenv("NATS_ENDPOINT"))
	err := cfg.SetObject(ServerConfig, server)
	if err != nil {
		return err
	}
	return nil
}
