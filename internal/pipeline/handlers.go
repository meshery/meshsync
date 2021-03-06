package pipeline

import (
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
			c.log.Info("received update event for:", obj.(*unstructured.Unstructured).GetName())
			c.publishItem(obj.(*unstructured.Unstructured), broker.Update)
		},
		DeleteFunc: func(obj interface{}) {
			c.log.Info("received delete event for:", obj.(*unstructured.Unstructured).GetName())
			c.publishItem(obj.(*unstructured.Unstructured), broker.Delete)
		},
	}
	s.AddEventHandler(handlers)
	s.Run(c.stopChan)
}

func (c *ResourceWatcher) publishItem(obj *unstructured.Unstructured, evtype broker.EventType) {
	err := c.brokerClient.Publish(c.config.PublishTo, &broker.Message{
		ObjectType: broker.Single,
		EventType:  evtype,
		Object:     model.ParseList(*obj),
	})
	if err != nil {
		c.log.Error(ErrPublish(c.config.Name, err))
	}
}
