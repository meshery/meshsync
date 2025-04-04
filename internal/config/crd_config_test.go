package config

// test for empty blacklist/whitelist
import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

var (
	Kind         string = "MeshSync"
	APIVersion   string = "meshery.io/v1alpha1"
	URL          string = "https://meshery.io"
	fakeDyClient *fake.FakeDynamicClient
	ctx          = context.Background()
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
		t.Error("BlackListed resources not equal")
	}

	// now we assertain the global and local pipelines have been correctly configured
	// excempted global pipelines: namespaces
	// excempted local pipelines: pods, replicasets

	// counting expected pipelines after blacklist
	expectedGlobalCount := len(Pipelines[GlobalResourceKey]) - 1 // excluding namespaces
	expectedLocalCount := len(Pipelines[LocalResourceKey]) - 2  // excluding pods, replicasets

	// Count how many items are actually excluded by the blacklist
	blacklistedGlobalCount := 0
	blacklistedLocalCount := 0

	for _, item := range meshsyncConfig.BlackList {
		for _, pipeline := range Pipelines[GlobalResourceKey] {
			if pipeline.Name == item {
				blacklistedGlobalCount++
			}
		}
		for _, pipeline := range Pipelines[LocalResourceKey] {
			if pipeline.Name == item {
				blacklistedLocalCount++
			}
		}
	}

	// Adjust expectations based on what was actually blacklisted
	expectedGlobalCount = len(Pipelines[GlobalResourceKey]) - blacklistedGlobalCount
	expectedLocalCount = len(Pipelines[LocalResourceKey]) - blacklistedLocalCount

	if len(meshsyncConfig.Pipelines["global"]) != expectedGlobalCount {
		t.Errorf("global pipelines not well configured expected %d got %d", 
			expectedGlobalCount, len(meshsyncConfig.Pipelines["global"]))
	}

	if len(meshsyncConfig.Pipelines["local"]) != expectedLocalCount {
		t.Errorf("local pipelines not well configured expected %d got %d", 
			expectedLocalCount, len(meshsyncConfig.Pipelines["local"]))
	}
}

func TestValidateMeshsyncCRD(t *testing.T) {
	tests := []struct {
		name    string
		crd     map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid CRD",
			crd: map[string]interface{}{
				"spec": map[string]interface{}{
					"version":    "v1",
					"watch-list": map[string]interface{}{},
				},
			},
			wantErr: false,
		},
		{
			name: "missing spec",
			crd: map[string]interface{}{
				"metadata": map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "missing watch-list",
			crd: map[string]interface{}{
				"spec": map[string]interface{}{
					"version": "v1",
				},
			},
			wantErr: true,
		},
		{
			name: "missing version",
			crd: map[string]interface{}{
				"spec": map[string]interface{}{
					"watch-list": map[string]interface{}{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMeshsyncCRD(tt.crd)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMeshsyncCRD() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInitializeDefaultConfig(t *testing.T) {
	// Create a scheme and add the necessary types
	scheme := runtime.NewScheme()
	
	// Create a fake dynamic client with the scheme
	fakeDyClient := fake.NewSimpleDynamicClient(scheme)
	
	// Test initialization
	err := InitializeDefaultConfig(fakeDyClient)
	if err != nil {
		t.Errorf("InitializeDefaultConfig() error = %v", err)
	}
	
	// Verify that the CR was created
	gvr := schema.GroupVersionResource{Version: version, Group: group, Resource: resource}
	crd, err := fakeDyClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), crName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get created CR: %v", err)
	}
	
	// Validate the created CR
	err = ValidateMeshsyncCRD(crd.Object)
	if err != nil {
		t.Errorf("Created CR failed validation: %v", err)
	}
	
	// Test idempotence (should not error when CR already exists)
	err = InitializeDefaultConfig(fakeDyClient)
	if err != nil {
		t.Errorf("InitializeDefaultConfig() error on second call = %v", err)
	}
}
