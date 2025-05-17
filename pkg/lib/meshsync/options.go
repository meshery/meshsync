package meshsync

import (
	mcp "github.com/layer5io/meshkit/config/provider"
)

type Options struct {
	MeshkitConfigProvider string
	Version               string
	PingEndpoint          string
}

var DefautOptions = Options{
	MeshkitConfigProvider: mcp.ViperKey,
	Version:               "Not Set",
	PingEndpoint:          ":8222/connz",
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
