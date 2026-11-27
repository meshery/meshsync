package model

const (
	// PlatformKubernetes is the DiscoveredResource.Platform discriminator for resources
	// discovered on a Kubernetes cluster.
	PlatformKubernetes = "kubernetes"

	// DiscoveredResourceSchemaVersion is the schema version of the DiscoveredResource envelope,
	// mirroring the discovered_resource construct (v1beta3) in meshery/schemas.
	DiscoveredResourceSchemaVersion = "discoveredresources.meshery.io/v1beta3"
)

// DiscoveredResource is the platform-agnostic envelope every discovery.Discoverer emits.
//
// It is a hand-written mirror of the discovered_resource construct in
// meshery/schemas; replacing it with the generated type is a separate, later
// change. Fields are camelCase on the wire per the ecosystem naming contract.
//
// Phase 0 note: DiscoveredResource is an in-process emit-time envelope only. The output
// writers still receive and publish the embedded KubernetesResource unchanged,
// so the broker message and snapshot file formats are not affected.
type DiscoveredResource struct {
	// ID is the platform-native identifier of the resource, when known.
	ID string `json:"id,omitempty"`
	// Platform is the discovery platform the resource was observed on
	// (for example "kubernetes").
	Platform string `json:"platform"`
	// ScopeID identifies the platform scope the resource belongs to: a
	// Kubernetes cluster UID, a Docker daemon ID, a cloud account and region.
	// It generalizes KubernetesResource.ClusterID.
	ScopeID string `json:"scopeId,omitempty"`
	// Kind is the resource kind within its platform (Pod, Container, ...).
	Kind string `json:"kind"`
	// Name is the resource name within its scope.
	Name string `json:"name,omitempty"`
	// Labels holds platform-native labels or tags as a flat string map.
	Labels map[string]string `json:"labels,omitempty"`
	// Metadata carries platform-specific data not otherwise modeled.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// ConnectionID is the Meshery connection the resource belongs to, once one
	// exists.
	ConnectionID string `json:"connectionId,omitempty"`
	// SchemaVersion is the version of this envelope's schema.
	SchemaVersion string `json:"schemaVersion"`

	// Kubernetes is the full legacy Kubernetes payload. It is set only when
	// Platform is PlatformKubernetes and is nil for every other platform.
	Kubernetes *KubernetesResource `json:"kubernetes,omitempty"`
}

// NewKubernetesDiscoveredResource wraps a parsed KubernetesResource in the
// platform-agnostic DiscoveredResource envelope.
//
// The scope identity is carried twice on purpose: DiscoveredResource.ScopeID is the new
// platform-agnostic field, and DiscoveredResource.Kubernetes.ClusterID (set by ParseList
// from the same cluster ID) keeps the legacy cluster_id field populated. Both
// hold the same value for one migration window so consumers can move from
// cluster_id to scopeId at their own pace. Do not drop either until every
// consumer has migrated.
//
// kr is stored as-is: the envelope never modifies the Kubernetes payload, so
// what the output writers publish is identical to what ParseList produced.
//
// Labels is filled from the Kubernetes metadata labels (not annotations), so a
// consumer reading the envelope directly sees the same labels the embedded
// payload carries.
func NewKubernetesDiscoveredResource(kr KubernetesResource, scopeID string) DiscoveredResource {
	name := ""
	labels := map[string]string{}
	if kr.KubernetesResourceMeta != nil {
		name = kr.KubernetesResourceMeta.Name
		for _, kv := range kr.KubernetesResourceMeta.Labels {
			if kv != nil {
				labels[kv.Key] = kv.Value
			}
		}
	}

	return DiscoveredResource{
		ID:            kr.ID,
		Platform:      PlatformKubernetes,
		ScopeID:       scopeID,
		Kind:          kr.Kind,
		Name:          name,
		Labels:        labels,
		SchemaVersion: DiscoveredResourceSchemaVersion,
		Kubernetes:    &kr,
	}
}
