package pipeline

import (
	"log"

	"github.com/layer5io/meshsync/internal/cache"
	"github.com/layer5io/meshsync/internal/model"
	broker "github.com/layer5io/meshsync/pkg/broker"
	discovery "github.com/layer5io/meshsync/pkg/discovery"
	"github.com/myntra/pipeline"
)

// Deployment will implement step interface for Deployments
type Deployment struct {
	pipeline.StepContext
	client *discovery.Client
	broker broker.Handler
}

// NewDeployment - constructor
func NewDeployment(client *discovery.Client, broker broker.Handler) *Deployment {
	return &Deployment{
		client: client,
		broker: broker,
	}
}

// Exec - step interface
func (d *Deployment) Exec(request *pipeline.Request) *pipeline.Result {
	// it will contain a pipeline to run
	log.Println("Deployment Discovery Started")

	// get all namespaces
	namespaces := cache.Storage["NamespaceNames"]

	for _, namespace := range namespaces {
		// get Deployments
		deployments, err := d.client.ListDeployments(namespace)
		if err != nil {
			return &pipeline.Result{
				Error: err,
			}
		}

		// processing
		for _, deployment := range deployments {
			// publishing discovered deployment
			err := d.broker.Publish(Subject, model.ConvModelObject(
				deployment.TypeMeta,
				deployment.ObjectMeta,
				deployment.Spec,
				deployment.Status,
			))
			if err != nil {
				log.Printf("Error publishing deployment named %s", deployment.Name)
			} else {
				log.Printf("Published deployment named %s", deployment.Name)
			}
		}
	}

	// no data is feeded to future steps or stages
	return &pipeline.Result{
		Error: nil,
	}
}

// Cancel - step interface
func (d *Deployment) Cancel() error {
	d.Status("cancel step")
	return nil
}
