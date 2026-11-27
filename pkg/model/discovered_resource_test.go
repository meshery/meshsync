package model

import (
	"encoding/json"
	"testing"

	"github.com/meshery/meshkit/broker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	discoveredResourceTestKind    = "Deployment"
	discoveredResourceTestName    = "frontend"
	discoveredResourceTestCluster = "cluster-1"
	discoveredResourceTestTier    = "storefront"
)

// newDiscoveredResourceTestObject builds an object with setters rather than a map
// literal, so it does not repeat the fixture strings other model tests use.
func newDiscoveredResourceTestObject() unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetAPIVersion("apps/v1")
	u.SetKind(discoveredResourceTestKind)
	u.SetName(discoveredResourceTestName)
	u.SetNamespace("shop")
	u.SetUID("frontend-uid")
	u.SetLabels(map[string]string{"tier": discoveredResourceTestTier})
	return u
}

func TestNewKubernetesResource(t *testing.T) {
	kr := ParseList(newDiscoveredResourceTestObject(), broker.Add, discoveredResourceTestCluster)

	res := NewKubernetesDiscoveredResource(kr, discoveredResourceTestCluster)

	assert.Equal(t, PlatformKubernetes, res.Platform)
	assert.Equal(t, DiscoveredResourceSchemaVersion, res.SchemaVersion)
	assert.Equal(t, discoveredResourceTestKind, res.Kind)
	assert.Equal(t, discoveredResourceTestName, res.Name)
	require.NotNil(t, res.Kubernetes)

	// Dual-write: the new scopeId and the legacy cluster_id carry the same value.
	assert.Equal(t, discoveredResourceTestCluster, res.ScopeID)
	assert.Equal(t, res.ScopeID, res.Kubernetes.ClusterID)

	// Labels carry over from the Kubernetes metadata into the flat envelope map.
	assert.Equal(t, map[string]string{"tier": discoveredResourceTestTier}, res.Labels)

	// The envelope must not change the Kubernetes payload in any way.
	assert.Equal(t, kr, *res.Kubernetes)
}

func TestNewKubernetesResourceNilMeta(t *testing.T) {
	res := NewKubernetesDiscoveredResource(KubernetesResource{Kind: discoveredResourceTestKind}, discoveredResourceTestCluster)

	assert.Empty(t, res.Name)
	assert.Empty(t, res.Labels)
	assert.Equal(t, discoveredResourceTestKind, res.Kind)
}

func TestDiscoveredResourceJSONRoundTrip(t *testing.T) {
	res := NewKubernetesDiscoveredResource(ParseList(newDiscoveredResourceTestObject(), broker.Add, discoveredResourceTestCluster), discoveredResourceTestCluster)

	data, err := json.Marshal(res)
	require.NoError(t, err)

	var fields map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &fields))
	assert.Equal(t, PlatformKubernetes, fields["platform"])
	assert.Equal(t, discoveredResourceTestCluster, fields["scopeId"])
	assert.Equal(t, DiscoveredResourceSchemaVersion, fields["schemaVersion"])
	assert.Contains(t, fields, PlatformKubernetes)

	// Compare re-marshaled JSON: empty slices tagged omitempty in the legacy
	// KubernetesResource decode as nil, which is equal on the wire.
	var decoded DiscoveredResource
	require.NoError(t, json.Unmarshal(data, &decoded))
	again, err := json.Marshal(decoded)
	require.NoError(t, err)
	assert.JSONEq(t, string(data), string(again))
}

func TestDiscoveredResourceJSONOmitsKubernetesWhenUnset(t *testing.T) {
	data, err := json.Marshal(DiscoveredResource{Platform: "docker", Kind: "Container", SchemaVersion: DiscoveredResourceSchemaVersion})
	require.NoError(t, err)

	assert.NotContains(t, string(data), `"kubernetes"`)
}
