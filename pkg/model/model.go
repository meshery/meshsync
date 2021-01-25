package model

type Object struct {
	Index      Index              `json:"index,omitempty"`
	TypeMeta   ResourceTypeMeta   `json:"typemeta,omitempty"`
	ObjectMeta ResourceObjectMeta `json:"metadata,omitempty"`
	Spec       ResourceSpec       `json:"spec,omitempty"`
	Status     ResourceStatus     `json:"status,omitempty"`
}

type Index struct {
	ID        string `json:"id,omitempty" gorm:"primarykey"`
	CreatedAt string `json:"created_at,omitempty" gorm:"index"`
	UpdatedAt string `json:"updated_at,omitempty" gorm:"index"`
	DeletedAt string `json:"deleted_at,omitempty" gorm:"index"`

	ResourceID   string `json:"resource-id,omitempty"`
	TypeMetaID   string `json:"type-meta-id,omitempty"`
	ObjectMetaID string `json:"object-meta-id,omitempty"`
	SpecID       string `json:"spec-id,omitempty"`
	StatusID     string `json:"status-id,omitempty"`
}

type ResourceTypeMeta struct {
	ID         string `json:"id,omitempty" gorm:"primarykey"`
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

type ResourceObjectMeta struct {
	ID                         string `json:"id,omitempty" gorm:"primarykey"`
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
	ID        string `json:"id,omitempty" gorm:"primarykey"`
	Attribute string `json:"attribute,omitempty" gorm:"type:json"`
}

type ResourceStatus struct {
	ID        string `json:"id,omitempty" gorm:"primarykey"`
	Attribute string `json:"attribute,omitempty" gorm:"type:json"`
}
