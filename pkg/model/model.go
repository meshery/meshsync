package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Object struct {
	ID        uuid.UUID `json:"id" gorm:"primarykey;type:uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt time.Time `json:"deleted_at" gorm:"index"`
	// ResourceID string              `json:"resource_id" gorm:"index"`
	APIVersion string              `json:"apiVersion" gorm:"index"`
	Kind       string              `json:"kind" gorm:"index"`
	ObjectMeta *ResourceObjectMeta `json:"metadata,omitempty" gorm:"foreignkey:ID;references:id"`
	Spec       *ResourceSpec       `json:"spec,omitempty" gorm:"foreignkey:ID;references:id"`
	Status     *ResourceStatus     `json:"status,omitempty" gorm:"foreignkey:ID;references:id"`

	// Secondary fields for configsmaps and secrets
	Immutable  string `json:"immutable,omitempty"`
	Data       string `json:"data,omitempty"`
	BinaryData string `json:"binaryData,omitempty"`
	StringData string `json:"stringData,omitempty"`
	Type       string `json:"type,omitempty"`
}

type KeyValue struct {
	ID       uuid.UUID `json:"id" gorm:"index;type:uuid"`
	UniqueID uuid.UUID `json:"unique_id" gorm:"primarykey;type:uuid"`
	Key      string    `json:"key,omitempty" gorm:"index"`
	Value    string    `json:"value,omitempty" gorm:"index"`
}

type ResourceObjectMeta struct {
	ID                         uuid.UUID   `json:"id" gorm:"primarykey;type:uuid"`
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
	Labels                     []*KeyValue `json:"labels,omitempty" gorm:"foreignkey:ID;references:id"`
	Annotations                []*KeyValue `json:"annotations,omitempty" gorm:"foreignkey:ID;references:id"`
	OwnerReferences            string      `json:"ownerReferences,omitempty" gorm:"type:json"`
	Finalizers                 string      `json:"finalizers,omitempty" gorm:"type:json"`
	ClusterName                string      `json:"clusterName,omitempty"`
	ManagedFields              string      `json:"managedFields,omitempty" gorm:"type:json"`
	ClusterID                  string      `json:"cluster_id"`
}

type ResourceSpec struct {
	ID        uuid.UUID `json:"id" gorm:"primarykey;type:uuid"`
	Attribute string    `json:"attribute,omitempty" gorm:"type:json"`
}

type ResourceStatus struct {
	ID        uuid.UUID `json:"id" gorm:"primarykey;type:uuid"`
	Attribute string    `json:"attribute,omitempty" gorm:"type:json"`
}

func (k *KeyValue) BeforeCreate(tx *gorm.DB) (err error) {
	k.UniqueID = uuid.New()
	return nil
}

func (k *Object) BeforeCreate(tx *gorm.DB) (err error) {
	k.ID = uuid.New()
	return nil
}

func (k *Object) AfterCreate(tx *gorm.DB) (err error) {
	k.CreatedAt = time.Now()
	return nil
}

func (k *Object) AfterSave(tx *gorm.DB) (err error) {
	k.UpdatedAt = time.Now()
	return nil
}

func (k *Object) AfterDelete(tx *gorm.DB) (err error) {
	k.DeletedAt = time.Now()
	return nil
}
