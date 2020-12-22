package informers

import (
	"log"

	"github.com/layer5io/meshsync/pkg/model"
	broker "github.com/layer5io/meshsync/pkg/broker"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Cluster) DeploymentInformer() cache.SharedIndexInformer {
	// get informer
	deploymentInformer := c.client.GetDeploymentInformer().Informer()

	// register event handlers
	deploymentInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				deployment := obj.(*v1.Deployment)
				log.Printf("Deployment Named: %s - added", deployment.Name)
				c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						deployment.TypeMeta,
						deployment.ObjectMeta,
						deployment.Spec,
						deployment.Status,
					)})
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				deployment := new.(*v1.Deployment)
				log.Printf("Deployment Named: %s - updated", deployment.Name)
				c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						deployment.TypeMeta,
						deployment.ObjectMeta,
						deployment.Spec,
						deployment.Status,
					)})
			},
			DeleteFunc: func(obj interface{}) {
				deployment := obj.(*v1.Deployment)
				log.Printf("Deployment Named: %s - deleted", deployment.Name)
				c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						deployment.TypeMeta,
						deployment.ObjectMeta,
						deployment.Spec,
						deployment.Status,
					)})
			},
		},
	)

	return deploymentInformer
}
