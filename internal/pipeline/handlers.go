package pipeline

import (
	"fmt"
	"strconv"
	"time"

	"github.com/layer5io/meshkit/broker"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

// type of the events that will be added to the workqueue
type QueueEvent struct {
	Obj    *unstructured.Unstructured
	EvType broker.EventType
	Config internalconfig.PipelineConfig
}

func (ri *RegisterInformer) registerHandlers(s cache.SharedIndexInformer) {

	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ri.queue.Add(QueueEvent{Obj: obj.(*unstructured.Unstructured), EvType: broker.Add, Config: ri.config})
			ri.log.Info("Added ADD event for: ", obj.(*unstructured.Unstructured).GetName(), "/", obj.(*unstructured.Unstructured).GetNamespace(), " to the queue")

		},
		UpdateFunc: func(oldObj, obj interface{}) {

			oldObjCasted := oldObj.(*unstructured.Unstructured)
			objCasted := obj.(*unstructured.Unstructured)

			oldRV, _ := strconv.ParseInt(oldObjCasted.GetResourceVersion(), 0, 64)
			newRV, _ := strconv.ParseInt(oldObjCasted.GetResourceVersion(), 0, 64)

			if oldRV < newRV {
				ri.queue.Add(QueueEvent{Obj: objCasted, EvType: broker.Update, Config: ri.config})
				ri.log.Info("Added UPDATE event for: ", obj.(*unstructured.Unstructured).GetName(), "/", obj.(*unstructured.Unstructured).GetNamespace(), " to the queue")
			} else {
				ri.log.Debug(fmt.Sprintf(
					"Skipping UPDATE event for: %s => [No changes detected]: %d %d",
					objCasted.GetName(),
					oldRV,
					newRV,
				))
			}
		},
		DeleteFunc: func(obj interface{}) {
			// the obj can only be of two types, Unstructured or DeletedFinalStateUnknown.
			// DeletedFinalStateUnknown means that the object that we receive may be `stale`
			// becuase of the way informer behaves

			// refer 'https://pkg.go.dev/k8s.io/client-go/tools/cache#ResourceEventHandler.OnDelete'

			var objCasted *unstructured.Unstructured
			objCasted = obj.(*unstructured.Unstructured)

			possiblyStaleObj, ok := obj.(cache.DeletedFinalStateUnknown)
			if ok {
				objCasted = possiblyStaleObj.Obj.(*unstructured.Unstructured)
			}
			ri.queue.Add(QueueEvent{Obj: objCasted, EvType: broker.Delete, Config: ri.config})
			ri.log.Info("Added DELETE event for: ", obj.(*unstructured.Unstructured).GetName(), "/", obj.(*unstructured.Unstructured).GetNamespace(), " to the queue")
		},
	}
	s.AddEventHandler(handlers)
}

func (pq *ProcessQueue) startProcessing() {

	// workerqueue provides us the guarantee that an item
	// will not be processed more than once concurrently
	// TODO: Configure multiple workers to improve performance
	go wait.Until(func() {
		for pq.processQueueItem() {
		}
	}, 1*time.Second, pq.stopChan)

}

func (pq *ProcessQueue) processQueueItem() bool {
	item, shutdown := pq.queue.Get()
	if shutdown {
		return false
	}

	defer pq.queue.Done(item) // to remove the item from the queue

	informerEvent, ok := item.(QueueEvent)

	var err error

	if !ok {
		err = fmt.Errorf("This type of event cannot be processed: %v", item)
		pq.log.Error(err)
	}

	switch informerEvent.EvType {
	case broker.Add:
		err = pq.publishItem(informerEvent.Obj, broker.Add, informerEvent.Config)
	case broker.Update:
		err = pq.publishItem(informerEvent.Obj, broker.Update, informerEvent.Config)
	case broker.Delete:
		err = pq.publishItem(informerEvent.Obj, broker.Delete, informerEvent.Config)
	}

	pq.handleError(err, item)

	return true

}

func (pq *ProcessQueue) handleError(err error, key interface{}) {
	if err == nil {
		pq.queue.Forget(key) // removes the item from retries list
		return
	}
	// retries for the given amount of time
	// TODO: Get the number of retires from the caller
	if pq.queue.NumRequeues(key) < 6 {
		pq.queue.AddRateLimited(key)
		return
	}
	pq.queue.Forget(key)
	pq.log.Error(fmt.Errorf("Dropping item: %v out of queue as it could be processed even after several retries", key))
}

func (pq *ProcessQueue) publishItem(obj *unstructured.Unstructured, evtype broker.EventType, config internalconfig.PipelineConfig) error {
	err := pq.brokerClient.Publish(config.PublishTo, &broker.Message{
		ObjectType: broker.MeshSync,
		EventType:  evtype,
		Object:     model.ParseList(*obj),
	})
	if err != nil {
		pq.log.Error(ErrPublish(config.Name, err))
		return err
	}

	return nil
}
