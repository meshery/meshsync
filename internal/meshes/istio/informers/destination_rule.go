package informers

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	"github.com/layer5io/meshsync/pkg/model"
	v1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/client-go/tools/cache"
)

func (i *Istio) DestinationRuleInformer() cache.SharedIndexInformer {
	// get informer
	DestinationRuleInformer := i.client.GetDestinationRuleInformer().Informer()

	// register event handlers
	DestinationRuleInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				DestinationRule := obj.(*v1beta1.DestinationRule)
				log.Printf("DestinationRule Named: %s - added", DestinationRule.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						DestinationRule.TypeMeta,
						DestinationRule.ObjectMeta,
						DestinationRule.Spec,
						DestinationRule.Status,
					)})
				if err != nil {
					log.Println("BROKER: Error publishing DestinationRule")
				}
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				DestinationRule := new.(*v1beta1.DestinationRule)
				log.Printf("DestinationRule Named: %s - updated", DestinationRule.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						DestinationRule.TypeMeta,
						DestinationRule.ObjectMeta,
						DestinationRule.Spec,
						DestinationRule.Status,
					)})
				if err != nil {
					log.Println("BROKER: Error publishing DestinationRule")
				}
			},
			DeleteFunc: func(obj interface{}) {
				DestinationRule := obj.(*v1beta1.DestinationRule)
				log.Printf("DestinationRule Named: %s - deleted", DestinationRule.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						DestinationRule.TypeMeta,
						DestinationRule.ObjectMeta,
						DestinationRule.Spec,
						DestinationRule.Status,
					)})
				if err != nil {
					log.Println("BROKER: Error publishing DestinationRule")
				}
			},
		},
	)

	return DestinationRuleInformer
}
