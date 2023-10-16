package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	KindLabel      string = "label"
	KindAnnotation string = "annotation"
)

type KubernetesObject struct {
	ID              string              `json:"id" gorm:"primarykey"`
	APIVersion      string              `json:"apiVersion" gorm:"index"`
	Kind            string              `json:"kind" gorm:"index"`
	KubernetesObjectMeta      *KubernetesResourceObjectMeta `json:"metadata" gorm:"foreignkey:ID;references:id;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Spec            *KubernetesResourceSpec       `json:"spec,omitempty" gorm:"foreignkey:ID;references:id;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Status          *KubernetesResourceStatus     `json:"status,omitempty" gorm:"foreignkey:ID;references:id;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	ClusterID       string              `json:"cluster_id"`
	PatternResource *uuid.UUID          `json:"pattern_resource"`

	// Secondary fields for configsmaps and secrets
	Immutable  string `json:"immutable,omitempty"`
	Data       string `json:"data,omitempty"`
	BinaryData string `json:"binaryData,omitempty"`
	StringData string `json:"stringData,omitempty"`
	Type       string `json:"type,omitempty"`
}

type KubernetesKeyValue struct {
	ID       string `json:"id" gorm:"primarykey"`
	UniqueID string `json:"unique_id" gorm:"index"`
	Kind     string `json:"kind" gorm:"primarykey"`
	Key      string `json:"key,omitempty" gorm:"primarykey"`
	Value    string `json:"value,omitempty" gorm:"primarykey"`
}

type KubernetesResourceObjectMeta struct {
	ID                         string      `json:"id" gorm:"primarykey"`
	Name                       string      `json:"name,omitempty" gorm:"index"`
	GenerateName               string      `json:"generateName,omitempty"`
	Namespace                  string      `json:"namespace,omitempty"`
	SelfLink                   string      `json:"selfLink,omitempty"`
	UID                        string      `json:"uid"`
	ResourceVersion            string      `json:"resourceVersion,omitempty"`
	Generation                 int64       `json:"generation,omitempty"`
	CreationTimestamp          string      `json:"creationTimestamp,omitempty"`
	DeletionTimestamp          string      `json:"deletionTimestamp,omitempty"`
	DeletionGracePeriodSeconds *int64      `json:"deletionGracePeriodSeconds,omitempty"`
	Labels                     []*KubernetesKeyValue `json:"labels,omitempty" gorm:"foreignkey:ID;references:id;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Annotations                []*KubernetesKeyValue `json:"annotations,omitempty" gorm:"foreignkey:ID;references:id;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	OwnerReferences            string      `json:"ownerReferences,omitempty" gorm:"-"`
	Finalizers                 string      `json:"finalizers,omitempty" gorm:"-"`
	ClusterName                string      `json:"clusterName,omitempty"`
	ManagedFields              string      `json:"managedFields,omitempty" gorm:"-"`
	ClusterID                  string      `json:"cluster_id"`
}

type KubernetesResourceSpec struct {
	ID        string `json:"id" gorm:"primarykey"`
	Attribute string `json:"attribute,omitempty"`
}

type KubernetesResourceStatus struct {
	ID        string `json:"id" gorm:"primarykey"`
	Attribute string `json:"attribute,omitempty"`
}

func (obj *KubernetesObject) BeforeCreate(tx *gorm.DB) (err error) {
	SetID(obj)
	return nil
}

func (obj *KubernetesObject) BeforeSave(tx *gorm.DB) (err error) {
	SetID(obj)
	return nil
}

func (obj *KubernetesObject) BeforeDelete(tx *gorm.DB) (err error) {
	SetID(obj)
	return nil
}
