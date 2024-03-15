package config

import (
	"reflect"
	"testing"

	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	discoveryFake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	testClient = fake.NewSimpleClientset()
)

func TestPopulateDefaultResources(t *testing.T) {

	// Add fake resources
	fakeResources := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "services", Namespaced: true, Kind: "Service"},
				{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
			},
		},
	}

	// mock the response from the discovery client
	testClient.Discovery().(*discoveryFake.FakeDiscovery).Resources = fakeResources
	discoveryClient := testClient.Discovery()

	PopulateDefaultResources(discoveryClient)
	if len(Pipelines[GlobalResourceKey]) != 1 {
		t.Error("global resources not discovered, expected 1")
	}

	if len(Pipelines[LocalResourceKey]) != 2 {
		t.Error("local resources not discovered, expected 1")
	}

	idx := slices.IndexFunc(Pipelines["global"], func(c PipelineConfig) bool { return c.Name == "namespaces.v1." })
	namespaces := Pipelines["global"][idx]
	if !reflect.DeepEqual(namespaces.Events, []string{"ADD", "UPDATE", "DELETE"}) {
		t.Errorf("failure propagating required events expected [ADDED,DELETE,UPDATE] found %v", namespaces.Events)
	}
}
