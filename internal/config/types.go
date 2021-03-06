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
	Name      string `json:"name" yaml:"name"`
	PublishTo string `json:"publish-to" yaml:"publish-to"`
}
