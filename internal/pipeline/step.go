package pipeline

import (
	"context"
	"sync"

	broker "github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/config"
	"github.com/layer5io/meshkit/logger"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/myntra/pipeline"

	"github.com/layer5io/meshsync/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type RegisterInformer struct {
	pipeline.StepContext
	log      logger.Handler
	informer dynamicinformer.DynamicSharedInformerFactory
	config   internalconfig.PipelineConfig
	broker   broker.Handler
}

func newRegisterInformerStep(log logger.Handler, informer dynamicinformer.DynamicSharedInformerFactory, config internalconfig.PipelineConfig, brkr broker.Handler) *RegisterInformer {
	return &RegisterInformer{
		log:      log,
		informer: informer,
		config:   config,
		broker:   brkr,
	}
}

// TODO: Find a way to respond when an informer has stopped for some reason unknown
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

type StartWatcher struct {
	pipeline.StepContext
	stopChan    chan struct{}
	dynamicKube dynamic.Interface
	log         logger.Handler
	broker      broker.Handler
	config      internalconfig.PipelineConfig
	hConfig     config.Handler
}

func newStartWatcherStage(dynamicKube dynamic.Interface, config internalconfig.PipelineConfig, stopChan chan struct{}, log logger.Handler, broker broker.Handler, hConfig config.Handler) *StartWatcher {
	return &StartWatcher{
		stopChan:    stopChan,
		dynamicKube: dynamicKube,
		config:      config,
		log:         log,
		broker:      broker,
		hConfig:     hConfig,
	}
}

func (w *StartWatcher) Exec(request *pipeline.Request) *pipeline.Result {

	var blacklist []string
	err := w.hConfig.GetObject("spec.informer_config", blacklist)
	if err != nil {
		return &pipeline.Result{
			Error: err,
			Data:  nil,
		}
	}

	b := true
	opts := metav1.ListOptions{
		Watch:                true,
		SendInitialEvents:    &b,
		ResourceVersionMatch: metav1.ResourceVersionMatchNotOlderThan,
	}

	if len(blacklist) != 0 {
		// Create a label selector to include all objects
		labelSelector := &metav1.LabelSelector{}
		// Add label selector requirements to exclude blacklisted types
		labelSelectorReq := metav1.LabelSelectorRequirement{
			Key:      "type",
			Operator: metav1.LabelSelectorOpNotIn,
			Values:   blacklist,
		}
		labelSelector.MatchExpressions = append(labelSelector.MatchExpressions, labelSelectorReq)

		opts.LabelSelector = labelSelector.String()
	}
	// attempts to begin watching the namespaces
	// returns a `watch.Interface`, or an error

	gvr, _ := schema.ParseResourceArg(w.config.Name)
	watcher, err := w.dynamicKube.Resource(*gvr).Watch(context.TODO(), opts)
	if err != nil {
		return &pipeline.Result{
			Error: err,
			Data:  nil,
		}
	}

	var wg sync.WaitGroup

	// Launch the goroutine and pass the channel as an argument
	wg.Add(1)
	go w.backgroundWatchProcessor(watcher.ResultChan(), w.stopChan, &wg)
	data := make(map[string]cache.Store)

	return &pipeline.Result{
		Error: nil,
		Data:  data,
	}
}

func (w *StartWatcher) backgroundWatchProcessor(result <-chan watch.Event, stopCh chan struct{}, wg *sync.WaitGroup) {
	for {
		select {
		case <-stopCh:
			return
		default:
			for event := range result {
				obj := event.Object
				switch event.Type {
				// when an event is added...
				case watch.Added:
					err := w.publishItem(obj.(*unstructured.Unstructured), broker.Add, w.config)
					if err != nil {
						w.log.Error(err)
					}
					w.log.Info("Received ADD event for: ", obj.(*unstructured.Unstructured).GetName(), "/", obj.(*unstructured.Unstructured).GetNamespace(), " of kind: ", obj.(*unstructured.Unstructured).GroupVersionKind().Kind)
				// when an event is modified...
				case watch.Modified:
					err := w.publishItem(obj.(*unstructured.Unstructured), broker.Update, w.config)
					if err != nil {
						w.log.Error(err)
					}
					w.log.Info("Received UPDATE event for: ", obj.(*unstructured.Unstructured).GetName(), "/", obj.(*unstructured.Unstructured).GetNamespace(), " of kind: ", obj.(*unstructured.Unstructured).GroupVersionKind().Kind)
				// when an event is deleted...
				case watch.Deleted:
					var objCasted *unstructured.Unstructured
					objCasted = obj.(*unstructured.Unstructured)
					err := w.publishItem(objCasted, broker.Delete, w.config)
					if err != nil {
						w.log.Error(err)
					}
					w.log.Info("Received DELETE event for: ", obj.(*unstructured.Unstructured).GetName(), "/", obj.(*unstructured.Unstructured).GetNamespace(), " of kind: ", obj.(*unstructured.Unstructured).GroupVersionKind().Kind)
				}
			}
		}
		if len(w.stopChan) > 0 {
			break
		}
	}
	wg.Wait()
}
func (w *StartWatcher) publishItem(obj *unstructured.Unstructured, evtype broker.EventType, config internalconfig.PipelineConfig) error {
	err := w.broker.Publish(config.PublishTo, &broker.Message{
		ObjectType: broker.MeshSync,
		EventType:  evtype,
		Object:     model.ParseList(*obj),
	})
	if err != nil {
		w.log.Error(ErrPublish(config.Name, err))
		return err
	}
	return nil
}

func (w *StartWatcher) Cancel() error {
	w.Status("cancel step")
	return nil
}
