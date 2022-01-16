package pipeline

import (
	broker "github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/logger"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/myntra/pipeline"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/util/workqueue"
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

	QueueProcessingStage = &pipeline.Stage{
		Name:       "QueueProcessing",
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}
)

func New(log logger.Handler, informer dynamicinformer.DynamicSharedInformerFactory, broker broker.Handler, plConfigs map[string]internalconfig.PipelineConfigs, stopChan chan struct{}, queue workqueue.RateLimitingInterface) *pipeline.Pipeline {

	// Global discovery
	gdstage := GlobalDiscoveryStage
	configs := plConfigs[gdstage.Name]
	for _, config := range configs {
		gdstage.AddStep(addResource(log, informer, broker, config, stopChan, queue)) // adds the events to the queue
	}

	// Local discovery
	ldstage := LocalDiscoveryStage
	configs = plConfigs[ldstage.Name]
	for _, config := range configs {
		ldstage.AddStep(addResource(log, informer, broker, config, stopChan, queue)) // adds the events to the queue
	}

	// Queue Processing
	qprcss := QueueProcessingStage
	qprcss.AddStep(newProcessQueueStep(stopChan, log, queue, broker, informer)) // Processes the events in the queue

	// Create Pipeline
	clusterPipeline := pipeline.New(Name, 1000)
	clusterPipeline.AddStage(gdstage)
	clusterPipeline.AddStage(ldstage)
	clusterPipeline.AddStage(qprcss)

	return clusterPipeline
}
