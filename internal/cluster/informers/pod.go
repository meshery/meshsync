package informers

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	"github.com/layer5io/meshsync/pkg/model"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Cluster) PodInformer() cache.SharedIndexInformer {
	// get informer
	podInformer := c.client.GetPodInformer().Informer()

	// register event handlers
	podInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				Pod := obj.(*v1.Pod)
				log.Printf("Pod Named: %s - added", Pod.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						Pod.TypeMeta,
						Pod.ObjectMeta,
						Pod.Spec,
						Pod.Status,
					)})
				if err != nil {
					log.Println("Error publishing Pod")
				}
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				Pod := new.(*v1.Pod)
				log.Printf("Pod Named: %s - updated", Pod.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						Pod.TypeMeta,
						Pod.ObjectMeta,
						Pod.Spec,
						Pod.Status,
					)})
				if err != nil {
					log.Println("Error publishing Pod")
				}
			},
			DeleteFunc: func(obj interface{}) {
				Pod := obj.(*v1.Pod)
				log.Printf("Pod Named: %s - deleted", Pod.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						Pod.TypeMeta,
						Pod.ObjectMeta,
						Pod.Spec,
						Pod.Status,
					)})
				if err != nil {
					log.Println("Error publishing Pod")
				}
			},
		},
	)

	return podInformer
}
