package informers

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	"github.com/layer5io/meshsync/pkg/model"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Cluster) NamespaceInformer() cache.SharedIndexInformer {
	// get informer
	namespaceInformer := c.client.GetNamespaceInformer().Informer()

	// register event handlers
	namespaceInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				Namespace := obj.(*v1.Namespace)
				log.Printf("Namespace Named: %s - added", Namespace.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						Namespace.TypeMeta,
						Namespace.ObjectMeta,
						Namespace.Spec,
						Namespace.Status,
					)})
				if err != nil {
					log.Println("Error publishing Namespace")
				}
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				Namespace := new.(*v1.Namespace)
				log.Printf("Namespace Named: %s - updated", Namespace.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						Namespace.TypeMeta,
						Namespace.ObjectMeta,
						Namespace.Spec,
						Namespace.Status,
					)})
				if err != nil {
					log.Println("Error publishing Namespace")
				}
			},
			DeleteFunc: func(obj interface{}) {
				Namespace := obj.(*v1.Namespace)
				log.Printf("Namespace Named: %s - deleted", Namespace.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						Namespace.TypeMeta,
						Namespace.ObjectMeta,
						Namespace.Spec,
						Namespace.Status,
					)})
				if err != nil {
					log.Println("Error publishing Namespace")
				}
			},
		},
	)

	return namespaceInformer
}
