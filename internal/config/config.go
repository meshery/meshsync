package config

import (
	"github.com/meshery/meshkit/config"
	configprovider "github.com/meshery/meshkit/config/provider"
	"github.com/meshery/meshkit/utils"
)

// New creates a new config instance
func New(provider string) (config.Handler, error) {
	var (
		handler config.Handler
		err     error
	)
	opts := configprovider.Options{
		// TODO do we need this always or only when running from binary?
		FilePath: utils.GetHome() + "/.meshery",
		FileType: "yaml",
		FileName: "meshsync_config",
	}

	// this is required, because if folder opts.Filepath does not exist
	// meshsync run ends up with error, f.e.
	// Error while initializing MeshSync configuration. .Config File \"meshsync_config\" Not Found in \"[/home/runner/.meshery]
	// if folder exists, there is no error
	if err := utils.CreateDirectory(opts.FilePath); err != nil {
		return nil, ErrInitConfig(err)
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

	return handler, nil
}
