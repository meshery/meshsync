package model

import (
	"github.com/layer5io/meshkit/database"
)

type Object struct {
	Index      Index              `json:"index,omitempty" gorm:"foreignKey:Index;references:ID"`
	TypeMeta   ResourceTypeMeta   `json:"typemeta,omitempty" gorm:"foreignKey:ResourceTypeMetaID;references:ID"`
	ObjectMeta ResourceObjectMeta `json:"metadata,omitempty" gorm:"foreignKey:ResourceObjectMetaID;references:ID"`
	Spec       ResourceSpec       `json:"spec,omitempty" gorm:"foreignKey:ResourceSpecID;references:ID"`
	Status     ResourceStatus     `json:"status,omitempty" gorm:"foreignKey:ResourceStatusID;references:ID"`
}

type Index struct {
	database.Model
	ResourceID   string `json:"resource-id,omitempty"`
	TypeMetaID   string `json:"type-meta-id,omitempty"`
	ObjectMetaID string `json:"object-meta-id,omitempty"`
	SpecID       string `json:"spec-id,omitempty"`
	StatusID     string `json:"status-id,omitempty"`
}

type ResourceTypeMeta struct {
	database.Model
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

type ResourceObjectMeta struct {
	database.Model
	Name                       string `json:"name,omitempty"`
	GenerateName               string `json:"generateName,omitempty"`
	Namespace                  string `json:"namespace,omitempty"`
	SelfLink                   string `json:"selfLink,omitempty"`
	UID                        string `json:"uid,omitempty"`
	ResourceVersion            string `json:"resourceVersion,omitempty"`
	Generation                 int64  `json:"generation,omitempty"`
	CreationTimestamp          string `json:"creationTimestamp,omitempty"`
	DeletionTimestamp          string `json:"deletionTimestamp,omitempty"`
	DeletionGracePeriodSeconds *int64 `json:"deletionGracePeriodSeconds,omitempty"`
	Labels                     string `json:"labels,omitempty" gorm:"type:json"`
	Annotations                string `json:"annotations,omitempty" gorm:"type:json"`
	// OwnerReferences            string `json:"ownerReferences,omitempty" gorm:"type:json"`
	// Finalizers                 string `json:"finalizers,omitempty" gorm:"type:json"`
	ClusterName string `json:"clusterName,omitempty"`
	// ManagedFields string `json:"managedFields,omitempty" gorm:"type:json"`
	ClusterID string `json:"cluster-id,omitempty"`
}

type ResourceSpec struct {
	database.Model
	Attribute string `json:"attribute,omitempty" gorm:"type:json"`
}

type ResourceStatus struct {
	database.Model
	Attribute string `json:"attribute,omitempty" gorm:"type:json"`
}
