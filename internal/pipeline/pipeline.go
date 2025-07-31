package pipeline

import (
	"strings"

	"github.com/meshery/meshkit/logger"
	internalconfig "github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/internal/output"
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

func New(
	log logger.Handler,
	informer dynamicinformer.DynamicSharedInformerFactory,
	ow output.Writer,
	plConfigs map[string]internalconfig.PipelineConfigs,
	stopChan chan struct{},
	clusterID string,
	outputFiltration internalconfig.OutputFiltrationContainer,
) *pipeline.Pipeline {
	// Global discovery
	gdstage := GlobalDiscoveryStage
	configs := plConfigs[gdstage.Name]
	for _, config := range configs {
		if shouldFiletrOutByName(config.Name, outputFiltration.ResourceSet) {
			// do not register informer for this config
			continue
		}
		gdstage.AddStep(newRegisterInformerStep(log, informer, config, ow, clusterID, outputFiltration)) // Register the informers for different resources
	}

	// Local discovery
	ldstage := LocalDiscoveryStage
	configs = plConfigs[ldstage.Name]
	for _, config := range configs {
		if shouldFiletrOutByName(config.Name, outputFiltration.ResourceSet) {
			// do not register informer for this config
			continue
		}

		ldstage.AddStep(newRegisterInformerStep(log, informer, config, ow, clusterID, outputFiltration)) // Register the informers for different resources
	}

	// Start informers
	strtInfmrs := StartInformersStage
	strtInfmrs.AddStep(newStartInformersStep(stopChan, log, informer)) // Start the registered informers

	// Create Pipeline
	clusterPipeline := pipeline.New(Name, 1000)
	if len(gdstage.Steps) > 0 {
		clusterPipeline.AddStage(gdstage)
	}
	if len(ldstage.Steps) > 0 {
		clusterPipeline.AddStage(ldstage)
	}
	clusterPipeline.AddStage(strtInfmrs)

	return clusterPipeline
}

func shouldFiletrOutByName(name string, resourcesSet internalconfig.OutputResourceSet) bool {
	if len(resourcesSet) == 0 {
		// only filter out if there are resources restriction provided
		return false
	}

	parts := strings.Split(name, ".")
	if len(parts) == 0 {
		// this is probably some invalid configuration,
		// but it is not related to our filtration, so do not filter out
		return false
	}

	return !resourcesSet.Contains(parts[0])
}
