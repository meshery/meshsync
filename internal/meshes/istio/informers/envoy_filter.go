package informers

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	"github.com/layer5io/meshsync/pkg/model"
	v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"k8s.io/client-go/tools/cache"
)

func (i *Istio) EnvoyFilterInformer() cache.SharedIndexInformer {
	// get informer
	EnvoyFilterInformer := i.client.GetEnvoyFilterInformer().Informer()

	// register event handlers
	EnvoyFilterInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				EnvoyFilter := obj.(*v1alpha3.EnvoyFilter)
				log.Printf("EnvoyFilter Named: %s - added", EnvoyFilter.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						EnvoyFilter.TypeMeta,
						EnvoyFilter.ObjectMeta,
						EnvoyFilter.Spec,
						EnvoyFilter.Status,
					)})
				if err != nil {
					log.Println("BROKER: Error publishing EnvoyFilter")
				}
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				EnvoyFilter := new.(*v1alpha3.EnvoyFilter)
				log.Printf("EnvoyFilter Named: %s - updated", EnvoyFilter.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						EnvoyFilter.TypeMeta,
						EnvoyFilter.ObjectMeta,
						EnvoyFilter.Spec,
						EnvoyFilter.Status,
					)})
				if err != nil {
					log.Println("BROKER: Error publishing EnvoyFilter")
				}
			},
			DeleteFunc: func(obj interface{}) {
				EnvoyFilter := obj.(*v1alpha3.EnvoyFilter)
				log.Printf("EnvoyFilter Named: %s - deleted", EnvoyFilter.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						EnvoyFilter.TypeMeta,
						EnvoyFilter.ObjectMeta,
						EnvoyFilter.Spec,
						EnvoyFilter.Status,
					)})
				if err != nil {
					log.Println("BROKER: Error publishing EnvoyFilter")
				}
			},
		},
	)

	return EnvoyFilterInformer
}
