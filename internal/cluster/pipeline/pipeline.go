package pipeline

import (
	broker "github.com/layer5io/meshsync/pkg/broker"
	discovery "github.com/layer5io/meshsync/pkg/discovery"
	"github.com/myntra/pipeline"
)

var (
	Name                 = "Cluster-Pipeline"
	GlobalDiscoveryStage = &pipeline.Stage{
		Name:       "Global-Resource-Discovery",
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}

	LocalDiscoveryStage = &pipeline.Stage{
		Name:       "Local-Resource-Discovery",
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}
	Subject = "meshsync"
)

func Initialize(client *discovery.Client, broker broker.Handler) *pipeline.Pipeline {
	// Global discovery
	gdstage := GlobalDiscoveryStage
	gdstage.AddStep(NewCluster(client, broker))
	gdstage.AddStep(NewNode(client, broker))
	gdstage.AddStep(NewNamespace(client, broker))

	// Local discovery
	ldstage := LocalDiscoveryStage
	ldstage.AddStep(NewDeployment(client, broker))
	ldstage.AddStep(NewService(client, broker))
	ldstage.AddStep(NewPod(client, broker))

	// Create Pipeline
	clusterPipeline := pipeline.New(Name, 1000)
	clusterPipeline.AddStage(gdstage)
	clusterPipeline.AddStage(ldstage)

	return clusterPipeline
}
