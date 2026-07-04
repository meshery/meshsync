package pipeline

import (
	"fmt"

	"github.com/meshery/meshkit/logger"
	internalconfig "github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/internal/output"
	"github.com/myntra/pipeline"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type RegisterInformer struct {
	pipeline.StepContext
	log              logger.Handler
	informer         dynamicinformer.DynamicSharedInformerFactory
	config           internalconfig.PipelineConfig
	outputWriter     output.Writer
	clusterID        string
	outputFiltration internalconfig.OutputFiltrationContainer
}

func newRegisterInformerStep(
	log logger.Handler,
	informer dynamicinformer.DynamicSharedInformerFactory,
	config internalconfig.PipelineConfig,
	ow output.Writer,
	clusterID string,
	outputFiltration internalconfig.OutputFiltrationContainer,
) *RegisterInformer {
	return &RegisterInformer{
		log:              log,
		informer:         informer,
		config:           config,
		outputWriter:     ow,
		clusterID:        clusterID,
		outputFiltration: outputFiltration,
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

// Exec starts the registered informers, then blocks until their caches have
// primed. The order is load-bearing: DynamicSharedInformerFactory.WaitForCacheSync
// only waits on informers that have already been started, so Start must run
// first. Calling WaitForCacheSync before Start makes it a no-op, so discovery
// proceeds against unprimed caches and reports an empty or partial cluster
// snapshot. WaitForCacheSync unblocks once every started informer has synced, or
// early if stopChan is closed (shutdown or resync).
func (si *StartInformers) Exec(request *pipeline.Request) *pipeline.Result {
	si.informer.Start(si.stopChan)

	var unsynced []string
	for gvr, ok := range si.informer.WaitForCacheSync(si.stopChan) {
		if !ok {
			unsynced = append(unsynced, gvr.String())
		}
	}
	if len(unsynced) > 0 {
		// A false sync result means stopChan closed before the cache primed - the
		// discovery was stopped or resynced mid-sync, which is expected teardown
		// rather than a hard failure, so it does not fail the pipeline step.
		si.log.Debugf("informer caches not primed before discovery stopped: %v", unsynced)
	} else {
		si.log.Debug("informer caches synced")
	}

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
