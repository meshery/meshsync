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
	resourceTestKind    = "Deployment"
	resourceTestName    = "frontend"
	resourceTestCluster = "cluster-1"
)

// newResourceTestObject builds an object with setters rather than a map
// literal, so it does not repeat the fixture strings other model tests use.
func newResourceTestObject() unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetAPIVersion("apps/v1")
	u.SetKind(resourceTestKind)
	u.SetName(resourceTestName)
	u.SetNamespace("shop")
	u.SetUID("frontend-uid")
	u.SetLabels(map[string]string{"tier": "storefront"})
	return u
}

func TestNewKubernetesResource(t *testing.T) {
	kr := ParseList(newResourceTestObject(), broker.Add, resourceTestCluster)

	res := NewKubernetesResource(kr, resourceTestCluster)

	assert.Equal(t, PlatformKubernetes, res.Platform)
	assert.Equal(t, ResourceSchemaVersion, res.SchemaVersion)
	assert.Equal(t, resourceTestKind, res.Kind)
	assert.Equal(t, resourceTestName, res.Name)
	require.NotNil(t, res.Kubernetes)

	// Dual-write: the new scopeId and the legacy cluster_id carry the same value.
	assert.Equal(t, resourceTestCluster, res.ScopeID)
	assert.Equal(t, res.ScopeID, res.Kubernetes.ClusterID)

	// The envelope must not change the Kubernetes payload in any way.
	assert.Equal(t, kr, *res.Kubernetes)
}

func TestNewKubernetesResourceNilMeta(t *testing.T) {
	res := NewKubernetesResource(KubernetesResource{Kind: resourceTestKind}, resourceTestCluster)

	assert.Empty(t, res.Name)
	assert.Equal(t, resourceTestKind, res.Kind)
}

func TestResourceJSONRoundTrip(t *testing.T) {
	res := NewKubernetesResource(ParseList(newResourceTestObject(), broker.Add, resourceTestCluster), resourceTestCluster)

	data, err := json.Marshal(res)
	require.NoError(t, err)

	var fields map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &fields))
	assert.Equal(t, PlatformKubernetes, fields["platform"])
	assert.Equal(t, resourceTestCluster, fields["scopeId"])
	assert.Equal(t, ResourceSchemaVersion, fields["schemaVersion"])
	assert.Contains(t, fields, PlatformKubernetes)

	// Compare re-marshaled JSON: empty slices tagged omitempty in the legacy
	// KubernetesResource decode as nil, which is equal on the wire.
	var decoded Resource
	require.NoError(t, json.Unmarshal(data, &decoded))
	again, err := json.Marshal(decoded)
	require.NoError(t, err)
	assert.JSONEq(t, string(data), string(again))
}

func TestResourceJSONOmitsKubernetesWhenUnset(t *testing.T) {
	data, err := json.Marshal(Resource{Platform: "docker", Kind: "Container", SchemaVersion: ResourceSchemaVersion})
	require.NoError(t, err)

	assert.NotContains(t, string(data), `"kubernetes"`)
}
