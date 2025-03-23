package pipeline

import (
	"fmt"
	"io"

	broker "github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/logger"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/myntra/pipeline"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type RegisterInformer struct {
	pipeline.StepContext
	log        logger.Handler
	informer   dynamicinformer.DynamicSharedInformerFactory
	config     internalconfig.PipelineConfig
	broker     broker.Handler
	fileWriter io.Writer
}

func newRegisterInformerStep(log logger.Handler, informer dynamicinformer.DynamicSharedInformerFactory, config internalconfig.PipelineConfig, brkr broker.Handler, fw io.Writer) *RegisterInformer {
	return &RegisterInformer{
		log:        log,
		informer:   informer,
		config:     config,
		broker:     brkr,
		fileWriter: fw,
	}
}

// TODO: Find a way to respond when an informer has stopped for some reason unknown
// Exec - step interface
func (ri *RegisterInformer) Exec(request *pipeline.Request) *pipeline.Result {
	gvr, _ := schema.ParseResourceArg(ri.config.Name)
	if gvr == nil {
		return &pipeline.Result{
			Error: internalconfig.ErrInitConfig(fmt.Errorf("error parsing resource arg, gvr not found")),
			Data:  nil,
		}
	}

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
func (ri *RegisterInformer) Cancel() error {
	ri.Status("cancel step")
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
	si.informer.WaitForCacheSync(si.stopChan)
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
