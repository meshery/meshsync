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
	MesheryResourceID    string
	ResourceID           string
	ResourceTypeMetaID   string
	ResourceObjectMetaID string
	ResourceSpecID       string
	ResourceStatusID     string
}

type KubernetesResourceTypeMeta struct {
	metav1.TypeMeta

	ResourceTypeMetaID string
}

type KubernetesResourceObjectMeta struct {
	metav1.ObjectMeta

	ResourceObjectMetaID string
	ClusterID            string
}

type KubernetesResourceSpec struct {
	ResourceSpecID string
	Attribute      map[string]interface{}
}

type KubernetesResourceStatus struct {
	ResourceStatusID string
	Attribute        map[string]interface{}
}
