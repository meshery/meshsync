package config

import (
	"flag"

	"golang.org/x/exp/slices"
)

const (
	ServerKey         = "server-config"
	PipelineNameKey   = "meshsync-pipeline"
	ResourcesKey      = "resources"
	GlobalResourceKey = "global"
	LocalResourceKey  = "local"
	ListenersKey      = "listeners"
	LogStreamsKey     = "log-streams"
	BrokerURL         = "broker-url"
	RequestStream     = "request-stream"
	LogStream         = "log-stream"
	ExecShell         = "exec-shell"
	InformerStore     = "informer-store"
)

// Command line flag to determine the output mode
var (
	OutputMode string
)

func init() {
	flag.StringVar(&OutputMode, "output", "nats", "Output mode: 'file' or 'nats'")
}
type PipelineConfigs []PipelineConfig

func (p PipelineConfigs) Add(pc PipelineConfig) PipelineConfigs {
	p = append(p, pc)
	return p
}

func (p PipelineConfigs) Delete(pc PipelineConfig) PipelineConfigs {
	for index, pipelineConfig := range p {
		if pipelineConfig.Name == pc.Name {
			p = slices.Delete[PipelineConfigs](p, index, index+1)
			break
		}
	}
	return p
}

type PipelineConfig struct {
	Name      string   `json:"name" yaml:"name"`
	PublishTo string   `json:"publish-to" yaml:"publish-to"`
	Events    []string `json:"events" yaml:"events"`
}

type ListenerConfigs []ListenerConfig

type ListenerConfig struct {
	Name           string   `json:"name" yaml:"name"`
	ConnectionName string   `json:"connection-name" yaml:"connection-name"`
	PublishTo      string   `json:"publish-to" yaml:"publish-to"`
	SubscribeTo    string   `json:"subscribe-to" yaml:"subscribe-to"`
	Events         []string `json:"events" yaml:"events"`
}

// Meshsync configuration controls the resources meshsync produces and consumes
type MeshsyncConfig struct {
	BlackList []string                   `json:"blacklist" yaml:"blacklist"`
	Pipelines map[string]PipelineConfigs `json:"pipeline-configs,omitempty" yaml:"pipeline-configs,omitempty"`
	Listeners map[string]ListenerConfig  `json:"listener-config,omitempty" yaml:"listener-config,omitempty"`
	WhiteList []ResourceConfig           `json:"resource-configs" yaml:"resource-configs"`
}

// Watched Resource configuration
type ResourceConfig struct {
	Resource string
	Events   []string
}
