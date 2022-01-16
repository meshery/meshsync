package pipeline

import (
	broker "github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/logger"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/myntra/pipeline"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type RegisterInformer struct {
	pipeline.StepContext
	log      logger.Handler
	informer dynamicinformer.DynamicSharedInformerFactory
	config   internalconfig.PipelineConfig
	queue    workqueue.RateLimitingInterface
}

func newRegisterInformerStep(log logger.Handler, informer dynamicinformer.DynamicSharedInformerFactory, config internalconfig.PipelineConfig, queue workqueue.RateLimitingInterface) *RegisterInformer {
	return &RegisterInformer{
		log:      log,
		informer: informer,
		config:   config,
		queue:    queue,
	}
}

// Exec - step interface
func (ri *RegisterInformer) Exec(request *pipeline.Request) *pipeline.Result {
	gvr, _ := schema.ParseResourceArg(ri.config.Name)
	iclient := ri.informer.ForResource(*gvr)

	ri.registerHandlers(iclient.Informer())

	// add the instance of store to the Result
	data := make(map[string]cache.Store)
	if request.Data != nil {
		data = request.Data.(map[string]cache.Store)
	}
	data[ri.config.Name] = iclient.Informer().GetStore()
	return &pipeline.Result{
		Error: nil,
		Data:  data,
	}
}

// Cancel - step interface
func (c *RegisterInformer) Cancel() error {
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
		Data:  request.Data,
	}
}

// Cancel - step interface
func (pq *ProcessQueue) Cancel() error {
	pq.Status("cancel step")
	return nil
}

// StartInformers Step

type StartInformers struct {
	pipeline.StepContext
	stopChan chan struct{}
	informer dynamicinformer.DynamicSharedInformerFactory
	log      logger.Handler
}

func newStartInformersStep(stopChan chan struct{}, log logger.Handler, informer dynamicinformer.DynamicSharedInformerFactory) *StartInformers {
	return &StartInformers{
		log:      log,
		informer: informer,
		stopChan: stopChan,
	}
}

func (si *StartInformers) Exec(request *pipeline.Request) *pipeline.Result {
	si.informer.Start(si.stopChan)
	return &pipeline.Result{
		Error: nil,
		Data:  request.Data,
	}
}

// Cancel - step interface
func (si *StartInformers) Cancel() error {
	si.Status("cancel step")
	return nil
}
