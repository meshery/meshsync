package config

// test for empty blacklist/whitelist
import (
	"context"
	"reflect"
	"testing"

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
				PublishTo: DefaultPublishingSubject,
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
			{
				Name:      "secrets.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
			{
				Name:      "persistentvolumes.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
			{
				Name:      "persistentvolumeclaims.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
		},
		LocalResourceKey: []PipelineConfig{
			// Core Resources
			{
				Name:      "replicasets.v1.apps",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "pods.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "services.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "deployments.v1.apps",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "statefulsets.v1.apps",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "daemonsets.v1.apps",
				PublishTo: DefaultPublishingSubject,
			},
			//Added Ingress support
			{
				Name:      "ingresses.v1.networking.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			// Added endpoint support
			{
				Name:      "endpoints.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			//Added endpointslice support
			{
				Name:      "endpointslices.v1.discovery.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			// Added cronJob support
			{
				Name:      "cronjobs.v1.batch",
				PublishTo: DefaultPublishingSubject,
			},
			//Added ReplicationController support
			{
				Name:      "replicationcontrollers.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			//Added storageClass support
			{
				Name:      "storageclasses.v1.storage.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added ClusterRole support
			{
				Name:      "clusterroles.v1.rbac.authorization.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added VolumeAttachment support
			{
				Name:      "volumeattachments.v1.storage.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added apiservice support
			{
				Name:      "apiservices.v1.apiregistration.k8s.io",
				PublishTo: DefaultPublishingSubject,
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

	if len(meshsyncConfig.Pipelines["global"]) != 1 {
		t.Error("global pipelines not well configured expected 1")
	}

	if len(meshsyncConfig.Pipelines["local"]) != 2 {
		t.Error("global pipelines not well configured expected 2")
	}
}

func TestBlackListResources(t *testing.T) {

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
		t.Errorf("WhiteListed resources")
	}

	expectedBlackList := []string{"namespaces.v1.", "pods.v1."}
	if !reflect.DeepEqual(meshsyncConfig.BlackList, expectedBlackList) {
		t.Error("WhiteListed resources not equal")
	}

	// now we assertain the global and local pipelines have been correctly configured
	// excempted global pipelines: namespaces
	// excempted local pipelines: pods, replicasets

	if len(meshsyncConfig.Pipelines["global"]) != 5 {
		t.Errorf("global pipelines not well configured got %d expected 6", len(meshsyncConfig.Pipelines["global"]))
	}

	if len(meshsyncConfig.Pipelines["local"]) != 14 {
		t.Errorf("global pipelines not well configured got %d expected 15", len(meshsyncConfig.Pipelines["local"]))
	}
}
