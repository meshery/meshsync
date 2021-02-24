package config

const (
	ServerKey         = "server-config"
	PipelineNameKey   = "meshsync-pipeline"
	ResourcesKey      = "resources"
	GlobalResourceKey = "global"
	LocalResourceKey  = "local"
	BrokerURL         = "broker-url"
)

type PipelineConfigs []PipelineConfig

type PipelineConfig struct {
	Group          string `json:"group" yaml:"group"`
	Version        string `json:"version" yaml:"version"`
	Resource       string `json:"resource" yaml:"resource"`
	Namespace      string `json:"namespace" yaml:"namespace"`
	PublishSubject string `json:"publish_subject" yaml:"publish_subject"`
}
