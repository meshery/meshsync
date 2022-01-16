package pipeline

import (
	broker "github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/logger"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/myntra/pipeline"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/util/workqueue"
)

type ResourceWatcher struct {
	pipeline.StepContext
	log          logger.Handler
	informer     dynamicinformer.DynamicSharedInformerFactory
	brokerClient broker.Handler
	config       internalconfig.PipelineConfig
	stopChan     chan struct{}
	queue        workqueue.RateLimitingInterface
}

func addResource(log logger.Handler, informer dynamicinformer.DynamicSharedInformerFactory, bclient broker.Handler, config internalconfig.PipelineConfig, stopChan chan struct{}, queue workqueue.RateLimitingInterface) *ResourceWatcher {
	return &ResourceWatcher{
		log:          log,
		informer:     informer,
		brokerClient: bclient,
		config:       config,
		stopChan:     stopChan,
		queue:        queue,
	}
}

// Exec - step interface
func (c *ResourceWatcher) Exec(request *pipeline.Request) *pipeline.Result {
	gvr, _ := schema.ParseResourceArg(c.config.Name)
	iclient := c.informer.ForResource(*gvr)

	go c.startWatching(iclient.Informer())

	return &pipeline.Result{
		Error: nil,
	}
}

// Cancel - step interface
func (c *ResourceWatcher) Cancel() error {
	c.Status("cancel step")
	return nil
}

// ProcessQueue Step

type ProcessQueue struct {
	pipeline.StepContext
	queue        workqueue.RateLimitingInterface
	brokerClient broker.Handler
	stopChan     chan struct{}
	log          logger.Handler
}

func newProcessQueueStep(stopChan chan struct{}, log logger.Handler, queue workqueue.RateLimitingInterface, bclient broker.Handler, informer dynamicinformer.DynamicSharedInformerFactory) *ProcessQueue {
	return &ProcessQueue{
		log:          log,
		brokerClient: bclient,
		queue:        queue,
		stopChan:     stopChan,
	}
}

func (pq *ProcessQueue) Exec(request *pipeline.Request) *pipeline.Result {
	go pq.startProcessing()

	return &pipeline.Result{
		Error: nil,
	}
}

// Cancel - step interface
func (pq *ProcessQueue) Cancel() error {
	pq.Status("cancel step")
	return nil
}
