package config

// test for empty blacklist/whitelist
import (
	"context"
	"reflect"
	"testing"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic/fake"
)

var (
	Kind         string = "MeshSync"
	APIVersion   string = "meshery.layer5.io/v1alpha1"
	URL          string = "https://layer5.io"
	fakeDyClient *fake.FakeDynamicClient
	ctx          = context.Background()
)

func TestWhiteListResources(t *testing.T) {

	Pipelines = map[string]PipelineConfigs{
		GlobalResourceKey: []PipelineConfig{
			// Core Resources
			{
				Name:      "namespaces.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
			{
				Name:      "configmaps.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
			{
				Name:      "nodes.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
		},
		LocalResourceKey: []PipelineConfig{
			// Core Resources
			{
				Name:      "replicasets.v1.apps",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
			{
				Name:      "pods.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
			{
				Name:      "services.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
		},
	}
	// Create an instance of the custom resource.
	watchList := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1apha1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-list",
			Namespace: "default",
		},
		Data: map[string]string{
			"blacklist": "",
			"whitelist": "[{\"Resource\":\"namespaces.v1.\",\"Events\":[\"ADDED\",\"DELETE\"]},{\"Resource\":\"replicasets.v1.apps\",\"Events\":[\"ADDED\",\"DELETE\"]},{\"Resource\":\"pods.v1.\",\"Events\":[\"MODIFIED\"]}]",
		},
	}

	meshsyncConfig, err := PopulateConfigs(watchList)

	if err != nil {
		t.Errorf("Meshsync config not well deserialized got %s", err.Error())
	}

	if len(meshsyncConfig.WhiteList) == 0 {
		t.Errorf("WhiteListed resources not correctly deserialized")
	}
	expectedWhiteList := []ResourceConfig{
		{Resource: "namespaces.v1.", Events: []string{"ADDED", "DELETE"}},
		{Resource: "replicasets.v1.apps", Events: []string{"ADDED", "DELETE"}},
		{Resource: "pods.v1.", Events: []string{"MODIFIED"}},
	}

	if !reflect.DeepEqual(meshsyncConfig.WhiteList, expectedWhiteList) {
		t.Error("WhiteListed resources not equal")
	}

	// now we assertain the global and local pipelines have been correctly configured
	// global pipelines: namespaces
	// local pipelines: pods, replicasets

	if len(Pipelines["global"]) != 3 {
		t.Error("global pipelines not well configured expected 3")
	}

	if len(Pipelines["local"]) != 3 {
		t.Error("global pipelines not well configured expected 3")
	}

	// granular test to ensure the required events are well propagated
	// namespaces expects two events
	idx := slices.IndexFunc(Pipelines["global"], func(c PipelineConfig) bool { return c.Name == "namespaces.v1." })
	namespaces := Pipelines["global"][idx]
	if !reflect.DeepEqual(namespaces.Events, []string{"ADDED", "DELETE"}) {
		t.Errorf("failure propagating required events expected [ADDED,DELETE] found %v", namespaces.Events)
	}
}

func TestBlackListResources(t *testing.T) {

	Pipelines = map[string]PipelineConfigs{
		GlobalResourceKey: []PipelineConfig{
			// Core Resources
			{
				Name:      "namespaces.v1.",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "configmaps.v1.",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "nodes.v1.",
				PublishTo: "meshery.meshsync.core",
			},
		},
		LocalResourceKey: []PipelineConfig{
			// Core Resources
			{
				Name:      "replicasets.v1.apps",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "pods.v1.",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "services.v1.",
				PublishTo: "meshery.meshsync.core",
			},
		},
	}
	// Create an instance of the custom resource.
	watchList := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1apha1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-list",
			Namespace: "default",
		},
		Data: map[string]string{
			"blacklist": "[\"namespaces.v1.\",\"pods.v1.\"]",
			"whitelist": "",
		},
	}

	meshsyncConfig, err := PopulateConfigs(watchList)

	if err != nil {
		t.Errorf("Meshsync config not well deserialized got %s", err.Error())
	}

	if len(meshsyncConfig.BlackList) == 0 {
		t.Errorf("Blacklisted resources missing")
	}

	expectedBlackList := []string{"namespaces.v1.", "pods.v1."}
	if !reflect.DeepEqual(meshsyncConfig.BlackList, expectedBlackList) {
		t.Error("Blacklisted resources not equal")
	}

	// now we assertain the global and local pipelines have been correctly configured
	// excempted global pipelines: namespaces
	// excempted local pipelines: pods, replicasets

	if len(Pipelines["global"]) != 2 {
		t.Error("global pipelines not well configured expected 2")
	}

	if len(Pipelines["local"]) != 2 {
		t.Error("global pipelines not well configured expected 2")
	}

	if idx := slices.IndexFunc(Pipelines["global"], func(c PipelineConfig) bool { return c.Name == "namespaces.v1." }); idx != -1 {
		t.Error("failed to remove blacklisted item, expected namespaces to be removed")
	}

	if idx := slices.IndexFunc(Pipelines["local"], func(c PipelineConfig) bool { return c.Name == "pods.v1." }); idx != -1 {
		t.Error("failed to remove blacklisted item, expected pods to be removed")
	}
}
