package pipeline

import (
	broker "github.com/layer5io/meshkit/broker"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/myntra/pipeline"
	"k8s.io/client-go/dynamic"
)

var (
	Name                 = internalconfig.PipelineNameKey
	GlobalDiscoveryStage = &pipeline.Stage{
		Name:       internalconfig.GlobalResourceKey,
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}

	LocalDiscoveryStage = &pipeline.Stage{
		Name:       internalconfig.LocalResourceKey,
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}
)

func New(client dynamic.Interface, broker broker.Handler, plConfigs map[string]internalconfig.PipelineConfigs) *pipeline.Pipeline {

	// Global discovery
	gdstage := GlobalDiscoveryStage
	configs := plConfigs[gdstage.Name]
	for _, config := range configs {
		gdstage.AddStep(NewGlobalResource(client, broker, config))
	}

	// Local discovery
	ldstage := LocalDiscoveryStage
	configs = plConfigs[ldstage.Name]
	for _, config := range configs {
		ldstage.AddStep(NewLocalResource(client, broker, config))
	}

	// Create Pipeline
	clusterPipeline := pipeline.New(Name, 1000)
	clusterPipeline.AddStage(gdstage)
	clusterPipeline.AddStage(ldstage)

	return clusterPipeline
}
