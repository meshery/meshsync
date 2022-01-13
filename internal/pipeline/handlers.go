package pipeline

import (
	"fmt"
	"strconv"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/pkg/model"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func (c *ResourceWatcher) startWatching(s cache.SharedIndexInformer) {
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.log.Info("received add event for:", obj.(*unstructured.Unstructured).GetName())
			c.publishItem(obj.(*unstructured.Unstructured), broker.Add)
		},
		UpdateFunc: func(oldObj, obj interface{}) {
			oldObjCasted := oldObj.(*unstructured.Unstructured)
			objCasted := obj.(*unstructured.Unstructured)

			oldRV, _ := strconv.ParseInt(oldObjCasted.GetResourceVersion(), 0, 64)
			newRV, _ := strconv.ParseInt(oldObjCasted.GetResourceVersion(), 0, 64)

			if oldRV < newRV {
				c.log.Info("received update event for:", objCasted.GetName())

				c.publishItem(objCasted, broker.Update)
			} else {
				c.log.Debug(fmt.Sprintf(
					"skipping update event for: %s => [No changes detected]: %d %d",
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
			c.log.Info("received delete event for:", objCasted.GetName())
			c.publishItem(objCasted, broker.Delete)
		},
	}
	s.AddEventHandler(handlers)
	s.Run(c.stopChan)
}

func (c *ResourceWatcher) publishItem(obj *unstructured.Unstructured, evtype broker.EventType) {
	err := c.brokerClient.Publish(c.config.PublishTo, &broker.Message{
		ObjectType: broker.MeshSync,
		EventType:  evtype,
		Object:     model.ParseList(*obj),
	})
	if err != nil {
		c.log.Error(ErrPublish(c.config.Name, err))
	}
}
