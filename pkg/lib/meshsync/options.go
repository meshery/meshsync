package meshsync

import (
	"time"

	mcp "github.com/layer5io/meshkit/config/provider"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/output"
)

type Options struct {
	OutputMode        string
	TransportChannel  chan<- *output.ChannelItem
	StopAfterDuration time.Duration

	Version               string
	PingEndpoint          string
	MeshkitConfigProvider string
}

var DefautOptions = Options{
	StopAfterDuration: -1, // -1 turns it off
	TransportChannel:  nil,

	Version:               "Not Set",
	PingEndpoint:          ":8222/connz",
	MeshkitConfigProvider: mcp.ViperKey,
}

var AllowedOutputModes = []string{
	config.OutputModeNats,
	config.OutputModeFile,
	config.OutputModeChannel,
}

type OptionsSetter func(*Options)

// value is one of the AllowedOutputModes
func WithOutputMode(value string) OptionsSetter {
	return func(o *Options) {
		o.OutputMode = value
	}
}

func WithTransportChannel(value chan<- *output.ChannelItem) OptionsSetter {
	return func(o *Options) {
		o.TransportChannel = value
	}
}

func WithStopAfterDuration(value time.Duration) OptionsSetter {
	return func(o *Options) {
		o.StopAfterDuration = value
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
