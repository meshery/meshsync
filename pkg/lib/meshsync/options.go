package meshsync

import (
	"time"

	mcp "github.com/layer5io/meshkit/config/provider"
	"github.com/layer5io/meshsync/internal/output"
)

type Options struct {
	Version               string
	PingEndpoint          string
	MeshkitConfigProvider string
	StopAfterDuration     time.Duration
	transportChannel      chan<- *output.ChannelItem
}

var DefautOptions = Options{
	Version:               "Not Set",
	PingEndpoint:          ":8222/connz",
	MeshkitConfigProvider: mcp.ViperKey,
	StopAfterDuration:     -1, // -1 turns it off
	transportChannel:      nil,
}

type OptionsSetter func(*Options)

func WithMeshkitConfigProvider(value string) OptionsSetter {
	return func(o *Options) {
		o.MeshkitConfigProvider = value
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

func WithStopAfterDuration(value time.Duration) OptionsSetter {
	return func(o *Options) {
		o.StopAfterDuration = value
	}
}

func WithTransportChannel(value chan<- *output.ChannelItem) OptionsSetter {
	return func(o *Options) {
		o.transportChannel = value
	}
}
