package pipeline

import (
	"strings"

	"github.com/meshery/meshkit/logger"
	internalconfig "github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/internal/output"
	"github.com/myntra/pipeline"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

var Name = internalconfig.PipelineNameKey

func New(
	log logger.Handler,
	informer dynamicinformer.DynamicSharedInformerFactory,
	ow output.Writer,
	plConfigs map[string]internalconfig.PipelineConfigs,
	stopChan chan struct{},
	clusterID string,
	outputFiltration internalconfig.OutputFiltrationContainer,
) *pipeline.Pipeline {
	// Stages are built fresh on every call. New runs once per discovery and again
	// on each resync, so the stages must not be shared package-level state: reusing
	// them would accumulate steps from prior runs that hold shut-down informer
	// factories and closed stop channels.

	// Global discovery
	gdstage := &pipeline.Stage{
		Name:       internalconfig.GlobalResourceKey,
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}
	for _, config := range plConfigs[gdstage.Name] {
		if shouldFilterOutByName(config.Name, outputFiltration.ResourceSet) {
			// do not register informer for this config
			continue
		}
		gdstage.AddStep(newRegisterInformerStep(log, informer, config, ow, clusterID, outputFiltration)) // Register the informers for different resources
	}

	// Local discovery
	ldstage := &pipeline.Stage{
		Name:       internalconfig.LocalResourceKey,
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}
	for _, config := range plConfigs[ldstage.Name] {
		if shouldFilterOutByName(config.Name, outputFiltration.ResourceSet) {
			// do not register informer for this config
			continue
		}

		ldstage.AddStep(newRegisterInformerStep(log, informer, config, ow, clusterID, outputFiltration)) // Register the informers for different resources
	}

	// Start informers
	strtInfmrs := &pipeline.Stage{
		Name:       "StartInformers",
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}
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

func shouldFilterOutByName(name string, resourcesSet internalconfig.OutputResourceSet) bool {
	if len(resourcesSet) == 0 {
		// only filter out if there are resources restriction provided
		return false
	}

	parts := strings.Split(name, ".")

	return !resourcesSet.Contains(parts[0])
}
