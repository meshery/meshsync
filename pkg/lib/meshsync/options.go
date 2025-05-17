package meshsync

import (
	"time"

	mcp "github.com/layer5io/meshkit/config/provider"
)

type Options struct {
	Version               string
	PingEndpoint          string
	MeshkitConfigProvider string
	StopAfterDuration     time.Duration
}

var DefautOptions = Options{
	Version:               "Not Set",
	PingEndpoint:          ":8222/connz",
	MeshkitConfigProvider: mcp.ViperKey,
	StopAfterDuration:     -1, // -1 turns it off
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
