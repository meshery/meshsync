package informers

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	"github.com/layer5io/meshsync/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Cluster) DeploymentInformer() cache.SharedIndexInformer {
	// get informer
	deploymentInformer := c.client.GetDeploymentInformer().Informer()

	// register event handlers
	deploymentInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				deployment := obj.(*appsv1.Deployment)
				log.Printf("Deployment Named: %s - added", deployment.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						metav1.TypeMeta{
							Kind:       "Deployment",
							APIVersion: "v1",
						},
						deployment.ObjectMeta,
						deployment.Spec,
						deployment.Status,
					)})
				if err != nil {
					log.Println("Error publishing Deployment")
				}
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				deployment := new.(*appsv1.Deployment)
				log.Printf("Deployment Named: %s - updated", deployment.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						metav1.TypeMeta{
							Kind:       "Deployment",
							APIVersion: "v1",
						},
						deployment.ObjectMeta,
						deployment.Spec,
						deployment.Status,
					)})
				if err != nil {
					log.Println("Error publishing Deployment")
				}
			},
			DeleteFunc: func(obj interface{}) {
				deployment := obj.(*appsv1.Deployment)
				log.Printf("Deployment Named: %s - deleted", deployment.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						metav1.TypeMeta{
							Kind:       "Deployment",
							APIVersion: "v1",
						},
						deployment.ObjectMeta,
						deployment.Spec,
						deployment.Status,
					)})
				if err != nil {
					log.Println("Error publishing Deployment")
				}
			},
		},
	)

	return deploymentInformer
}
