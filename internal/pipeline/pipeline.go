package pipeline

import (
	"github.com/layer5io/meshkit/logger"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/output"
	"github.com/myntra/pipeline"
	"k8s.io/client-go/dynamic/dynamicinformer"
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

	StartInformersStage = &pipeline.Stage{
		Name:       "StartInformers",
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}
)

func New(log logger.Handler, informer dynamicinformer.DynamicSharedInformerFactory, ow output.Writer, plConfigs map[string]internalconfig.PipelineConfigs, stopChan chan struct{}) *pipeline.Pipeline {
	// Global discovery
	gdstage := GlobalDiscoveryStage
	configs := plConfigs[gdstage.Name]
	for _, config := range configs {
		gdstage.AddStep(newRegisterInformerStep(log, informer, config, ow)) // Register the informers for different resources
	}

	// Local discovery
	ldstage := LocalDiscoveryStage
	configs = plConfigs[ldstage.Name]
	for _, config := range configs {
		ldstage.AddStep(newRegisterInformerStep(log, informer, config, ow)) // Register the informers for different resources
	}

	// Start informers
	strtInfmrs := StartInformersStage
	strtInfmrs.AddStep(newStartInformersStep(stopChan, log, informer)) // Start the registered informers

	// Create Pipeline
	clusterPipeline := pipeline.New(Name, 1000)
	clusterPipeline.AddStage(gdstage)
	clusterPipeline.AddStage(ldstage)
	clusterPipeline.AddStage(strtInfmrs)

	return clusterPipeline
}
