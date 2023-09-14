package pipeline

import (
	broker "github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/logger"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/myntra/pipeline"
	"k8s.io/client-go/dynamic"
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

func New(log logger.Handler, informer dynamicinformer.DynamicSharedInformerFactory, broker broker.Handler, plConfigs map[string]internalconfig.PipelineConfigs, stopChan chan struct{}, dynamicKube dynamic.Interface) *pipeline.Pipeline {
	// TODO: best way to check whether WatchList feature is enabled
	watchList := true

	// Global discovery
	gdstage := GlobalDiscoveryStage
	configs := plConfigs[gdstage.Name]
	if watchList {
		for _, config := range configs {
			gdstage.AddStep(newStartWatcherStage(dynamicKube, config, stopChan, log, broker)) // Register the watchers for different resources
		}
	} else {
		for _, config := range configs {
			gdstage.AddStep(newRegisterInformerStep(log, informer, config, broker)) // Register the informers for different resources
		}
	}

	// Local discovery
	ldstage := LocalDiscoveryStage
	configs = plConfigs[ldstage.Name]

	if watchList {
		for _, config := range configs {
			ldstage.AddStep(newStartWatcherStage(dynamicKube, config, stopChan, log, broker)) // Register the watchers for different resources
		}
	} else {
		for _, config := range configs {
			ldstage.AddStep(newRegisterInformerStep(log, informer, config, broker)) // Register the informers for different resources
		}
	}

	// Create Pipeline
	clusterPipeline := pipeline.New(Name, 1000)

	// add stages to pipeline
	clusterPipeline.AddStage(gdstage)
	clusterPipeline.AddStage(ldstage)
	if !watchList {
		// Start informers
		strtInfmrs := StartInformersStage
		strtInfmrs.AddStep(newStartInformersStep(stopChan, log, informer)) // Start the registered informers
		clusterPipeline.AddStage(strtInfmrs)
	}
	return clusterPipeline
}
