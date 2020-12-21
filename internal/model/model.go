package model

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Object struct {
	Resource   KubernetesResource
	TypeMeta   KubernetesResourceTypeMeta
	ObjectMeta KubernetesResourceObjectMeta
	Spec       KubernetesResourceSpec
	Status     KubernetesResourceStatus
}

type KubernetesResource struct {
	MesheryResourceID    string `json:"meshery-resource-id,omitempty"`
	ResourceID           string `json:"resource-id,omitempty"`
	ResourceTypeMetaID   string `json:"resource-type-meta-id,omitempty"`
	ResourceObjectMetaID string `json:"resource-object-meta-id,omitempty"`
	ResourceSpecID       string `json:"resource-spec-id,omitempty"`
	ResourceStatusID     string `json:"resource-status-id,omitempty"`
}

type KubernetesResourceTypeMeta struct {
	metav1.TypeMeta `json:",inline"`

	ResourceTypeMetaID string `json:"resource-type-meta-id,omitempty"`
}

type KubernetesResourceObjectMeta struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ResourceObjectMetaID string `json:"resource-object-meta-id,omitempty"`
	ClusterID            string `json:"cluster-id,omitempty"`
}

type KubernetesResourceSpec struct {
	ResourceSpecID string                 `json:"resource-spec-id,omitempty"`
	Attribute      map[string]interface{} `json:"attribute,omitempty"`
}

type KubernetesResourceStatus struct {
	ResourceStatusID string                 `json:"resource-status-id,omitempty"`
	Attribute        map[string]interface{} `json:"attribute,omitempty"`
}
