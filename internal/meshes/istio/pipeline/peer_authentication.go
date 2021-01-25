package pipeline

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	discovery "github.com/layer5io/meshsync/pkg/discovery"
	"github.com/layer5io/meshsync/pkg/model"
	"github.com/myntra/pipeline"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PeerAuthentication will implement step interface for PeerAuthentications
type PeerAuthentication struct {
	pipeline.StepContext
	// clients
	client *discovery.Client
	broker broker.Handler
}

// NewPeerAuthentication - constructor
func NewPeerAuthentication(client *discovery.Client, broker broker.Handler) *PeerAuthentication {
	return &PeerAuthentication{
		client: client,
		broker: broker,
	}
}

// Exec - step interface
func (pa *PeerAuthentication) Exec(request *pipeline.Request) *pipeline.Result {
	// it will contain a pipeline to run
	log.Println("PeerAuthentication Discovery Started")

	for _, namespace := range Namespaces {
		peerAuthentications, err := pa.client.ListPeerAuthentications(namespace)
		if err != nil {
			return &pipeline.Result{
				Error: err,
			}
		}

		// processing
		for _, peerAuthentication := range peerAuthentications {
			// publishing discovered peerAuthentication
			err := pa.broker.Publish(Subject, &broker.Message{
				Object: model.ConvObject(
					metav1.TypeMeta{
						Kind:       "PeerAuthentication",
						APIVersion: "v1beta1",
					},
					peerAuthentication.ObjectMeta,
					peerAuthentication.Spec,
					peerAuthentication.Status,
				)})
			if err != nil {
				log.Printf("Error publishing peer authentication named %s", peerAuthentication.Name)
			} else {
				log.Printf("Published peer authentication named %s", peerAuthentication.Name)
			}
		}
	}

	// no data is feeded to future steps or stages
	return &pipeline.Result{
		Error: nil,
	}
}

// Cancel - step interface
func (pa *PeerAuthentication) Cancel() error {
	pa.Status("cancel step")
	return nil
}
