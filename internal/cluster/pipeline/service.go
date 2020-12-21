package pipeline

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	discovery "github.com/layer5io/meshsync/pkg/discovery"
	"github.com/myntra/pipeline"
)

// Service will implement step interface for Services
type Service struct {
	pipeline.StepContext
	client *discovery.Client
	broker broker.Handler
}

// NewService - constructor
func NewService(client *discovery.Client, broker broker.Handler) *Service {
	return &Service{
		client: client,
		broker: broker,
	}
}

// Exec - step interface
func (d *Service) Exec(request *pipeline.Request) *pipeline.Result {
	// it will contain a pipeline to run
	log.Println("Service Discovery Started")

	// get all namespaces
	namespaces := NamespaceName

	for _, namespace := range namespaces {
		// get Services
		services, err := d.client.ListServices(namespace)
		if err != nil {
			return &pipeline.Result{
				Error: err,
			}
		}

		// processing
		for _, service := range services {
			// publishing discovered Service
			err := d.broker.Publish(Subject, broker.Message{
				Object: service,
			})
			if err != nil {
				log.Printf("Error publishing service named %s", service.Name)
			} else {
				log.Printf("Published service named %s", service.Name)
			}
		}
	}

	// no data is feeded to future steps or stages
	return &pipeline.Result{
		Error: nil,
	}
}

// Cancel - step interface
func (d *Service) Cancel() error {
	d.Status("cancel step")
	return nil
}
