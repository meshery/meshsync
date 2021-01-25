package informers

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	"github.com/layer5io/meshsync/pkg/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Cluster) ServiceInformer() cache.SharedIndexInformer {
	// get informer
	serviceInformer := c.client.GetServiceInformer().Informer()

	// register event handlers
	serviceInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				service := obj.(*corev1.Service)
				log.Printf("Service Named: %s - added", service.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						metav1.TypeMeta{
							Kind:       "Service",
							APIVersion: "v1",
						},
						service.ObjectMeta,
						service.Spec,
						service.Status,
					)})
				if err != nil {
					log.Println("Error publishing Service")
				}
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				service := new.(*corev1.Service)
				log.Printf("Service Named: %s - updated", service.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						service.TypeMeta,
						service.ObjectMeta,
						service.Spec,
						service.Status,
					)})
				if err != nil {
					log.Println("Error publishing Service")
				}
			},
			DeleteFunc: func(obj interface{}) {
				service := obj.(*corev1.Service)
				log.Printf("Service Named: %s - deleted", service.Name)
				err := c.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						service.TypeMeta,
						service.ObjectMeta,
						service.Spec,
						service.Status,
					)})
				if err != nil {
					log.Println("Error publishing Service")
				}
			},
		},
	)

	return serviceInformer
}
