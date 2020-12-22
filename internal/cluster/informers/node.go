package informers

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	"github.com/layer5io/meshsync/pkg/model"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Cluster) NodeInformer() cache.SharedIndexInformer {
	// get informer
	nodeInformer := c.client.GetNodeInformer().Informer()

	// register event handlers
	nodeInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				Node := obj.(*v1.Node)
				log.Printf("Node Named: %s - added", Node.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						Node.TypeMeta,
						Node.ObjectMeta,
						Node.Spec,
						Node.Status,
					)})
				if err != nil {
					log.Println("Error publishing Node")
				}
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				Node := new.(*v1.Node)
				log.Printf("Node Named: %s - updated", Node.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						Node.TypeMeta,
						Node.ObjectMeta,
						Node.Spec,
						Node.Status,
					)})
				if err != nil {
					log.Println("Error publishing Node")
				}
			},
			DeleteFunc: func(obj interface{}) {
				Node := obj.(*v1.Node)
				log.Printf("Node Named: %s - deleted", Node.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						Node.TypeMeta,
						Node.ObjectMeta,
						Node.Spec,
						Node.Status,
					)})
				if err != nil {
					log.Println("Error publishing Node")
				}
			},
		},
	)

	return nodeInformer
}
