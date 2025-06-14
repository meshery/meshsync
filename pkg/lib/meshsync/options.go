package meshsync

import (
	"time"

	"github.com/meshery/meshkit/broker"
	mcp "github.com/meshery/meshkit/config/provider"
	"github.com/meshery/meshsync/internal/config"
)

type Options struct {
	OutputMode        string
	StopAfterDuration time.Duration
	KubeConfig        []byte
	OutputFileName    string
	BrokerHandler     broker.Handler

	Version               string
	PingEndpoint          string
	MeshkitConfigProvider string
}

var DefautOptions = Options{
	StopAfterDuration: -1,  // -1 turns it off
	KubeConfig:        nil, // if nil, truies to detekt kube config by the means of github.com/meshery/meshkit/utils/kubernetes/client.go:DetectKubeConfig
	BrokerHandler:     nil, // if nil, will instantiate broker connection itself

	Version:               "Not Set",
	PingEndpoint:          ":8222/connz",
	MeshkitConfigProvider: mcp.ViperKey,
}

var AllowedOutputModes = []string{
	config.OutputModeNats,
	config.OutputModeFile,
}

type OptionsSetter func(*Options)

// value is one of the AllowedOutputModes
func WithOutputMode(value string) OptionsSetter {
	return func(o *Options) {
		o.OutputMode = value
	}
}

func WithStopAfterDuration(value time.Duration) OptionsSetter {
	return func(o *Options) {
		o.StopAfterDuration = value
	}
}

// value here is all what is good to pass to github.com/meshery/meshkit/utils/kubernetes/client.go:DetectKubeConfig
func WithKubeConfig(value []byte) OptionsSetter {
	return func(o *Options) {
		o.KubeConfig = value
	}
}

func WithOutputFileName(value string) OptionsSetter {
	return func(o *Options) {
		o.OutputFileName = value
	}
}

func WithBrokerHandler(value broker.Handler) OptionsSetter {
	return func(o *Options) {
		o.BrokerHandler = value
	}
}

func WithVersion(value string) OptionsSetter {
	return func(o *Options) {
		o.Version = value
	}
}

func WithPingEndpoint(value string) OptionsSetter {
	return func(o *Options) {
		o.PingEndpoint = value
	}
}

func WithMeshkitConfigProvider(value string) OptionsSetter {
	return func(o *Options) {
		o.MeshkitConfigProvider = value
	}
}
