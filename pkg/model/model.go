package model

import (
	"gorm.io/gorm"
)

const (
	KindLabel      string = "label"
	KindAnnotation string = "annotation"
)

type Object struct {
	ID         string              `json:"id" gorm:"primarykey"`
	APIVersion string              `json:"apiVersion" gorm:"index"`
	Kind       string              `json:"kind" gorm:"index"`
	ObjectMeta *ResourceObjectMeta `json:"metadata" gorm:"foreignkey:ID;references:id;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Spec       *ResourceSpec       `json:"spec,omitempty" gorm:"foreignkey:ID;references:id;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Status     *ResourceStatus     `json:"status,omitempty" gorm:"foreignkey:ID;references:id;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	// Secondary fields for configsmaps and secrets
	Immutable  string `json:"immutable,omitempty"`
	Data       string `json:"data,omitempty"`
	BinaryData string `json:"binaryData,omitempty"`
	StringData string `json:"stringData,omitempty"`
	Type       string `json:"type,omitempty"`
}

type KeyValue struct {
	ID       string `json:"id" gorm:"index"`
	UniqueID string `json:"unique_id" gorm:"primarykey"`
	Kind     string `json:"kind" gorm:"index"`
	Key      string `json:"key,omitempty" gorm:"index"`
	Value    string `json:"value,omitempty" gorm:"index"`
}

type ResourceObjectMeta struct {
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
	Labels                     []*KeyValue `json:"labels,omitempty" gorm:"foreignkey:ID;references:id;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Annotations                []*KeyValue `json:"annotations,omitempty" gorm:"foreignkey:ID;references:id;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	OwnerReferences            string      `json:"ownerReferences,omitempty" gorm:"type:json"`
	Finalizers                 string      `json:"finalizers,omitempty" gorm:"type:json"`
	ClusterName                string      `json:"clusterName,omitempty"`
	ManagedFields              string      `json:"managedFields,omitempty" gorm:"type:json"`
	ClusterID                  string      `json:"cluster_id"`
}

type ResourceSpec struct {
	ID        string `json:"id" gorm:"primarykey"`
	Attribute string `json:"attribute,omitempty" gorm:"type:json"`
}

type ResourceStatus struct {
	ID        string `json:"id" gorm:"primarykey"`
	Attribute string `json:"attribute,omitempty" gorm:"type:json"`
}

func (obj *Object) BeforeCreate(tx *gorm.DB) (err error) {
	SetID(obj)
	return nil
}

func (obj *Object) BeforeSave(tx *gorm.DB) (err error) {
	SetID(obj)
	return nil
}
