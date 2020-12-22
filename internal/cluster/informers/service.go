package informers

import (
	"log"

	"github.com/layer5io/meshsync/internal/model"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Cluster) ServiceInformer() cache.SharedIndexInformer {
	// get informer
	serviceInformer := c.client.GetServiceInformer().Informer()

	// register event handlers
	serviceInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				service := obj.(*v1.Service)
				log.Printf("Service Named: %s - added", service.Name)
				c.broker.Publish(Subject, model.ConvModelObject(
					service.TypeMeta,
					service.ObjectMeta,
					service.Spec,
					service.Status,
				))
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				service := new.(*v1.Service)
				log.Printf("Service Named: %s - updated", service.Name)
				c.broker.Publish(Subject, model.ConvModelObject(
					service.TypeMeta,
					service.ObjectMeta,
					service.Spec,
					service.Status,
				))
			},
			DeleteFunc: func(obj interface{}) {
				service := obj.(*v1.Service)
				log.Printf("Service Named: %s - deleted", service.Name)
				c.broker.Publish(Subject, model.ConvModelObject(
					service.TypeMeta,
					service.ObjectMeta,
					service.Spec,
					service.Status,
				))
			},
		},
	)

	return serviceInformer
}
