package pipeline

import (
	"log"

	broker "github.com/layer5io/meshsync/pkg/broker"
	discovery "github.com/layer5io/meshsync/pkg/discovery"
	"github.com/layer5io/meshsync/pkg/model"
	"github.com/myntra/pipeline"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkloadEntry will implement step interface for WorkloadEntry
type WorkloadEntry struct {
	pipeline.StepContext
	client *discovery.Client
	broker broker.Handler
}

// NewWOrkloadEntry - constructor
func NewWorkloadEntry(client *discovery.Client, broker broker.Handler) *WorkloadEntry {
	return &WorkloadEntry{
		client: client,
		broker: broker,
	}
}

// Exec - step interface
func (we *WorkloadEntry) Exec(request *pipeline.Request) *pipeline.Result {
	// it will contain a pipeline to run
	log.Println("Workload  Entry Discovery Started")

	// Workload  Entry for all namespace
	for _, namespace := range Namespaces {
		workloadEntries, err := we.client.ListWorkloadEntries(namespace)
		if err != nil {
			return &pipeline.Result{
				Error: err,
			}
		}

		// processing
		for _, workloadEntry := range workloadEntries {
			// publishing discovered workloadEntry
			err := we.broker.Publish(Subject, &broker.Message{
				Object: model.ConvObject(
					metav1.TypeMeta{
						Kind:       "WorkloadEntry",
						APIVersion: "v1beta1",
					},
					workloadEntry.ObjectMeta,
					workloadEntry.Spec,
					workloadEntry.Status,
				)})
			if err != nil {
				log.Printf("Error publishing workload entry named %s", workloadEntry.Name)
			} else {
				log.Printf("Published workload entry named %s", workloadEntry.Name)
			}
		}
	}

	// no data is feeded to future steps or stages
	return &pipeline.Result{
		Error: nil,
	}
}

// Cancel - step interface
func (we *WorkloadEntry) Cancel() error {
	we.Status("cancel step")
	return nil
}
