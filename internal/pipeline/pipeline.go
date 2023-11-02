package pipeline

import (
	"context"

	broker "github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/config"
	"github.com/layer5io/meshkit/logger"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/myntra/pipeline"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

var (
	Name                 = internalconfig.PipelineNameKey
	GlobalDiscoveryStage = &pipeline.Stage{
		Name:       internalconfig.GlobalResourceKey,
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}

	LocalDiscoveryStage = &pipeline.Stage{
		Name:       internalconfig.LocalResourceKey,
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}

	StartInformersStage = &pipeline.Stage{
		Name:       "StartInformers",
		Concurrent: false,
		Steps:      []pipeline.Step{},
	}
)

func New(log logger.Handler, informer dynamicinformer.DynamicSharedInformerFactory, broker broker.Handler, plConfigs map[string]internalconfig.PipelineConfigs, stopChan chan struct{}, dynamicKube dynamic.Interface, hConfig config.Handler) *pipeline.Pipeline {
	// TODO: best way to check whether WatchList feature is enabled
	watchList := checkWatchListFeatureBruteForce(dynamicKube)

	// Global discovery
	gdstage := GlobalDiscoveryStage
	configs := plConfigs[gdstage.Name]
	if watchList {
		for _, config := range configs {
			gdstage.AddStep(newStartWatcherStage(dynamicKube, config, stopChan, log, broker, hConfig, informer)) // Register the watchers for different resources
		}
	} else {
		for _, config := range configs {
			gdstage.AddStep(newRegisterInformerStep(log, informer, config, broker)) // Register the informers for different resources
		}
	}

	// Local discovery
	ldstage := LocalDiscoveryStage
	configs = plConfigs[ldstage.Name]

	if watchList {
		for _, config := range configs {
			ldstage.AddStep(newStartWatcherStage(dynamicKube, config, stopChan, log, broker, hConfig, informer)) // Register the watchers for different resources
		}
	} else {
		for _, config := range configs {
			ldstage.AddStep(newRegisterInformerStep(log, informer, config, broker)) // Register the informers for different resources
		}
	}

	// Create Pipeline
	clusterPipeline := pipeline.New(Name, 1000)
	clusterPipeline.AddStage(gdstage)
	clusterPipeline.AddStage(ldstage)
	if !watchList {
		// Start informers
		strtInfmrs := StartInformersStage
		strtInfmrs.AddStep(newStartInformersStep(stopChan, log, informer)) // Start the registered informers
		clusterPipeline.AddStage(strtInfmrs)
	}
	return clusterPipeline
}

// checkWatchListFeatureBruteForce checks if the WatchList feature is present by doing a test
// streaming list watch command on a simple pod and watching the result, a positive result
// means the feature is enabled
func checkWatchListFeatureBruteForce(client dynamic.Interface) bool {
	b := true
	opts := metav1.ListOptions{
		Watch:                true,
		SendInitialEvents:    &b,
		ResourceVersionMatch: metav1.ResourceVersionMatchNotOlderThan,
	}
	gvr, _ := schema.ParseResourceArg("pods.v1.")
	_, err := client.Resource(*gvr).Watch(context.TODO(), opts)

	return err == nil
}
