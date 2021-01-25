package informers

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	"github.com/layer5io/meshsync/pkg/model"
	v1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func (i *Istio) PeerAuthenticationInformer() cache.SharedIndexInformer {
	// get informer
	PeerAuthenticationInformer := i.client.GetPeerAuthenticationInformer().Informer()

	// register event handlers
	PeerAuthenticationInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				PeerAuthentication := obj.(*v1beta1.PeerAuthentication)
				log.Printf("PeerAuthentication Named: %s - added", PeerAuthentication.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						metav1.TypeMeta{
							Kind:       "PeerAuthentication",
							APIVersion: "v1beta1",
						},
						PeerAuthentication.ObjectMeta,
						PeerAuthentication.Spec,
						PeerAuthentication.Status,
					)})
				if err != nil {
					log.Println("BROKER: Error publishing PeerAuthentication")
				}
			},
			UpdateFunc: func(new interface{}, old interface{}) {
				PeerAuthentication := new.(*v1beta1.PeerAuthentication)
				log.Printf("PeerAuthentication Named: %s - updated", PeerAuthentication.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						metav1.TypeMeta{
							Kind:       "PeerAuthentication",
							APIVersion: "v1beta1",
						},
						PeerAuthentication.ObjectMeta,
						PeerAuthentication.Spec,
						PeerAuthentication.Status,
					)})
				if err != nil {
					log.Println("BROKER: Error publishing PeerAuthentication")
				}
			},
			DeleteFunc: func(obj interface{}) {
				PeerAuthentication := obj.(*v1beta1.PeerAuthentication)
				log.Printf("PeerAuthentication Named: %s - deleted", PeerAuthentication.Name)
				err := i.broker.Publish(Subject, &broker.Message{
					Object: model.ConvObject(
						metav1.TypeMeta{
							Kind:       "PeerAuthentication",
							APIVersion: "v1beta1",
						},
						PeerAuthentication.ObjectMeta,
						PeerAuthentication.Spec,
						PeerAuthentication.Status,
					)})
				if err != nil {
					log.Println("BROKER: Error publishing PeerAuthentication")
				}
			},
		},
	)

	return PeerAuthenticationInformer
}
