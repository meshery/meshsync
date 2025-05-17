package meshsync

import (
	configprovider "github.com/layer5io/meshkit/config/provider"
)

type Options struct {
	Provider     string
	Version      string
	PingEndpoint string
}

var DefautOptions = Options{
	Provider:     configprovider.ViperKey,
	Version:      "Not Set",
	PingEndpoint: ":8222/connz",
}

type OptionsSetter func(*Options)

func WithProvider(value string) OptionsSetter {
	return func(o *Options) {
		o.Provider = value
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
