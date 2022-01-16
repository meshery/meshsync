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

func (c *RegisterInformer) registerHandlers(s cache.SharedIndexInformer) {

	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.queue.Add(QueueEvent{Obj: obj.(*unstructured.Unstructured), EvType: broker.Add, Config: c.config})
			c.log.Info("Added ADD event for:", obj.(*unstructured.Unstructured).GetName(), "to the queue")

		},
		UpdateFunc: func(oldObj, obj interface{}) {

			oldObjCasted := oldObj.(*unstructured.Unstructured)
			objCasted := obj.(*unstructured.Unstructured)

			oldRV, _ := strconv.ParseInt(oldObjCasted.GetResourceVersion(), 0, 64)
			newRV, _ := strconv.ParseInt(oldObjCasted.GetResourceVersion(), 0, 64)

			if oldRV < newRV {
				c.queue.Add(QueueEvent{Obj: objCasted, EvType: broker.Update, Config: c.config})
				c.log.Info("Added UPDATE event for:", obj.(*unstructured.Unstructured).GetName(), " to the queue")
			} else {
				c.log.Debug(fmt.Sprintf(
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
			c.queue.Add(QueueEvent{Obj: objCasted, EvType: broker.Delete, Config: c.config})
			c.log.Info("Added DELETE event for:", objCasted.GetName(), " to the queue")
		},
	}
	s.AddEventHandler(handlers)
}

func (pq *ProcessQueue) startProcessing() {

	// workerqueue provides us the guarantee that an item
	// will not be processed more than once concurrently
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

	defer pq.queue.Forget(item) // to remove the item from the retries list
	defer pq.queue.Done(item)   // to remove the item from the queue
	informerEvent, ok := item.(QueueEvent)
	if !ok {
		pq.log.Error(fmt.Errorf("This type of event cannot be processed: %v", item))
		// TODO:what to do when the event format is invalid ?
		return true
	}

	switch informerEvent.EvType {
	case broker.Add:
		pq.publishItem(informerEvent.Obj, broker.Add, informerEvent.Config)
	case broker.Update:
		pq.publishItem(informerEvent.Obj, broker.Update, informerEvent.Config)
	case broker.Delete:
		pq.publishItem(informerEvent.Obj, broker.Delete, informerEvent.Config)
	}

	return true

}

func (pq *ProcessQueue) publishItem(obj *unstructured.Unstructured, evtype broker.EventType, config internalconfig.PipelineConfig) {
	err := pq.brokerClient.Publish(config.PublishTo, &broker.Message{
		ObjectType: broker.MeshSync,
		EventType:  evtype,
		Object:     model.ParseList(*obj),
	})
	if err != nil {
		pq.log.Error(ErrPublish(config.Name, err))
	}
}
