package config

// test for empty blacklist/whitelist
import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	Kind       string = "MeshSync"
	APIVersion string = "meshery.io/v1alpha1"
	URL        string = "https://meshery.io"
)

func TestWhiteListResources(t *testing.T) {

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
			"blacklist": "[\"namespaces.v1.\",\"replicasets.v1.apps\",\"pods.v1.\"]",
			"whitelist": "",
		},
	}

	meshsyncConfig, err := PopulateConfigs(watchList)

	if err != nil {
		t.Errorf("Meshsync config not well deserialized got %s", err.Error())
	}

	if len(meshsyncConfig.BlackList) == 0 {
		t.Errorf("Blacklisted resources not correctly deserialized")
	}

	expectedBlackList := []string{"namespaces.v1.", "replicasets.v1.apps", "pods.v1."}
	if !reflect.DeepEqual(meshsyncConfig.BlackList, expectedBlackList) {
		t.Error("BlackListed resources not equal")
	}

	// now we assertain the global and local pipelines have been correctly configured
	// excempted global pipelines: namespaces
	// excempted local pipelines: pods, replicasets

	// counting expected pipelines after blacklist
	expectedGlobalCount := len(Pipelines[GlobalResourceKey]) - 1 // excluding namespaces
	expectedLocalCount := len(Pipelines[LocalResourceKey]) - 2   // excluding pods, replicasets

	if len(meshsyncConfig.Pipelines["global"]) != expectedGlobalCount {
		t.Errorf("global pipelines not well configured expected %d", expectedGlobalCount)
	}

	if len(meshsyncConfig.Pipelines["local"]) != expectedLocalCount {
		t.Errorf("local pipelines not well configured expected %d", expectedLocalCount)
	}
}
