package pipeline

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	discovery "github.com/layer5io/meshsync/pkg/discovery"
	"github.com/layer5io/meshsync/pkg/model"
	"github.com/myntra/pipeline"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvoyFilter will implement step interface for EnvoyFilters
type EnvoyFilter struct {
	pipeline.StepContext
	// clients
	client *discovery.Client
	broker broker.Handler
}

// NewEnvoyFilter - constructor
func NewEnvoyFilter(client *discovery.Client, broker broker.Handler) *EnvoyFilter {
	return &EnvoyFilter{
		client: client,
		broker: broker,
	}
}

// Exec - step interface
func (ef *EnvoyFilter) Exec(request *pipeline.Request) *pipeline.Result {
	// it will contain a pipeline to run
	log.Println("EnvoyFilter Discovery Started")

	for _, namespace := range Namespaces {
		envoyFilters, err := ef.client.ListEnvoyFilters(namespace)
		if err != nil {
			return &pipeline.Result{
				Error: err,
			}
		}

		// processing
		for _, envoyFilter := range envoyFilters {
			// publishing discovered envoyFilter
			err := ef.broker.Publish(Subject, &broker.Message{
				Object: model.ConvObject(
					metav1.TypeMeta{
						Kind:       "EnvoyFilter",
						APIVersion: "v1beta1",
					},
					envoyFilter.ObjectMeta,
					envoyFilter.Spec,
					envoyFilter.Status,
				)})
			if err != nil {
				log.Printf("Error publishing envoy filter named %s", envoyFilter.Name)
			} else {
				log.Printf("Published envoy filter named %s", envoyFilter.Name)
			}
		}
	}

	// no data is feeded to future steps or stages
	return &pipeline.Result{
		Error: nil,
	}
}

// Cancel - step interface
func (ef *EnvoyFilter) Cancel() error {
	ef.Status("cancel step")
	return nil
}
