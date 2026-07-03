package config

// test for empty blacklist/whitelist
import (
	"reflect"
	"testing"

	"github.com/meshery/meshkit/broker"
	"golang.org/x/exp/slices"
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

// TestDefaultEventsMatchBrokerWireTypes pins DefaultEvents to the broker's wire
// event types. publishItem gates every event on
// slices.Contains(config.Events, string(evtype)) where evtype is one of
// broker.Add/Update/Delete, so any drift here silently drops events for every
// pipeline that inherits DefaultEvents (the blacklist path and the default
// pipelines).
func TestDefaultEventsMatchBrokerWireTypes(t *testing.T) {
	want := []string{string(broker.Add), string(broker.Update), string(broker.Delete)}
	if !reflect.DeepEqual(DefaultEvents, want) {
		t.Fatalf("DefaultEvents = %v, want broker wire types %v", DefaultEvents, want)
	}

	// Assert the concrete wire values too, so a rename of the broker constants
	// that changed their string values would fail loudly here rather than
	// silently breaking discovery.
	if !reflect.DeepEqual(DefaultEvents, []string{"ADDED", "MODIFIED", "DELETED"}) {
		t.Fatalf("DefaultEvents = %v, want [ADDED MODIFIED DELETED]", DefaultEvents)
	}
}

// TestBlackListResourcesUseBrokerEvents is the regression test for the
// blacklist silent-drop bug: filterBlacklistedPipelines stamps DefaultEvents
// onto every blacklist-derived pipeline, and those events must be the broker
// wire types or publishItem drops the resource entirely.
func TestBlackListResourcesUseBrokerEvents(t *testing.T) {
	watchList := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "watch-list", Namespace: "default"},
		Data: map[string]string{
			"blacklist": `["namespaces.v1."]`,
			"whitelist": "",
		},
	}

	meshsyncConfig, err := PopulateConfigs(watchList)
	if err != nil {
		t.Fatalf("PopulateConfigs returned error: %v", err)
	}

	wantEvents := []string{string(broker.Add), string(broker.Update), string(broker.Delete)}
	sawPipeline := false
	for key, pipelines := range meshsyncConfig.Pipelines {
		for _, p := range pipelines {
			sawPipeline = true
			if !reflect.DeepEqual(p.Events, wantEvents) {
				t.Errorf("blacklist pipeline %q in %q has Events %v, want %v", p.Name, key, p.Events, wantEvents)
			}
			// Guard against a regression to the old, never-matching literals.
			for _, bad := range []string{"ADD", "UPDATE", "DELETE"} {
				if slices.Contains(p.Events, bad) {
					t.Errorf("blacklist pipeline %q uses non-broker event %q: %v", p.Name, bad, p.Events)
				}
			}
		}
	}
	if !sawPipeline {
		t.Fatal("expected blacklist config to produce pipelines, got none")
	}
}
