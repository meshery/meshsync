package pipeline

import (
	broker "github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/logger"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/myntra/pipeline"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

type ResourceWatcher struct {
	pipeline.StepContext
	log          logger.Handler
	informer     dynamicinformer.DynamicSharedInformerFactory
	brokerClient broker.Handler
	config       internalconfig.PipelineConfig
	stopChan     chan struct{}
}

func addResource(log logger.Handler, informer dynamicinformer.DynamicSharedInformerFactory, bclient broker.Handler, config internalconfig.PipelineConfig, stopChan chan struct{}) *ResourceWatcher {
	return &ResourceWatcher{
		log:          log,
		informer:     informer,
		brokerClient: bclient,
		config:       config,
		stopChan:     stopChan,
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
