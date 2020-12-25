package informers

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	"github.com/layer5io/meshsync/pkg/model"
	v1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/client-go/tools/cache"
)

func (i *Istio) ServiceEntryInformer() cache.SharedIndexInformer {
	// get informer
	ServiceEntryInformer := i.client.GetServiceEntryInformer().Informer()

	// register event handlers
	ServiceEntryInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ServiceEntry := obj.(*v1beta1.ServiceEntry)
				log.Printf("ServiceEntry Named: %s - added", ServiceEntry.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						ServiceEntry.TypeMeta,
						ServiceEntry.ObjectMeta,
						ServiceEntry.Spec,
						ServiceEntry.Status,
					)})
				if err != nil {
					log.Println("NATS: Error publishing ServiceEntry")
				}
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				ServiceEntry := new.(*v1beta1.ServiceEntry)
				log.Printf("ServiceEntry Named: %s - updated", ServiceEntry.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						ServiceEntry.TypeMeta,
						ServiceEntry.ObjectMeta,
						ServiceEntry.Spec,
						ServiceEntry.Status,
					)})
				if err != nil {
					log.Println("NATS: Error publishing ServiceEntry")
				}
			},
			DeleteFunc: func(obj interface{}) {
				ServiceEntry := obj.(*v1beta1.ServiceEntry)
				log.Printf("ServiceEntry Named: %s - deleted", ServiceEntry.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						ServiceEntry.TypeMeta,
						ServiceEntry.ObjectMeta,
						ServiceEntry.Spec,
						ServiceEntry.Status,
					)})
				if err != nil {
					log.Println("NATS: Error publishing ServiceEntry")
				}
			},
		},
	)

	return ServiceEntryInformer
}
