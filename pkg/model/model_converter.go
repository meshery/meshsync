package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshsync/internal/cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConvObject(typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta, spec interface{}, status interface{}) Object {
	resourceIdentifier := fmt.Sprintf("%s-%s-%s-%s", typeMeta.Kind, typeMeta.APIVersion, objectMeta.Namespace, objectMeta.Name)
	index := generateIndex(resourceIdentifier)
	resourceTypeMeta := makeTypeMeta(typeMeta, index.TypeMetaID)
	resourceObjectMeta := makeObjectMeta(objectMeta, index.ObjectMetaID)
	resourceSpec := makeSpec(spec, index.SpecID)
	resourceStatus := makeStatus(status, index.StatusID)

	return Object{
		Index:      index,
		TypeMeta:   resourceTypeMeta,
		ObjectMeta: resourceObjectMeta,
		Spec:       resourceSpec,
		Status:     resourceStatus,
	}
}

func generateIndex(id string) Index {
	return Index{
		ID:           id,
		CreatedAt:    time.Now().String(),
		ResourceID:   uuid.New().String(),
		TypeMetaID:   uuid.New().String(),
		ObjectMetaID: uuid.New().String(),
		SpecID:       uuid.New().String(),
		StatusID:     uuid.New().String(),
	}
}

func makeTypeMeta(resource metav1.TypeMeta, id string) ResourceTypeMeta {
	return ResourceTypeMeta{
		ID:         id,
		Kind:       resource.Kind,
		APIVersion: resource.APIVersion,
	}
}

func makeObjectMeta(resource metav1.ObjectMeta, id string) ResourceObjectMeta {
	labels, _ := utils.Marshal(resource.Labels)
	annotations, _ := utils.Marshal(resource.Annotations)

	var creationTime string
	var deletionTime string
	if !resource.CreationTimestamp.IsZero() {
		creationTime = resource.CreationTimestamp.String()
	}
	if !resource.DeletionTimestamp.IsZero() {
		deletionTime = resource.DeletionTimestamp.String()
	}

	return ResourceObjectMeta{
		ID:                         id,
		Name:                       resource.Name,
		GenerateName:               resource.GenerateName,
		Namespace:                  resource.Namespace,
		SelfLink:                   resource.SelfLink,
		UID:                        string(resource.UID),
		ResourceVersion:            resource.ResourceVersion,
		Generation:                 resource.Generation,
		CreationTimestamp:          creationTime,
		DeletionTimestamp:          deletionTime,
		DeletionGracePeriodSeconds: resource.DeletionGracePeriodSeconds,
		Labels:                     labels,
		Annotations:                annotations,
		// OwnerReferences:            resource.OwnerReferences,
		// Finalizers:  resource.Finalizers,
		ClusterName: resource.ClusterName,
		// ManagedFields:              resource.ManagedFields,
		ClusterID: cache.ClusterID,
	}
}

func makeSpec(spec interface{}, id string) ResourceSpec {
	specJSON, _ := utils.Marshal(spec)

	return ResourceSpec{
		ID:        id,
		Attribute: string(specJSON),
	}
}

func makeStatus(status interface{}, id string) ResourceStatus {
	statusJSON, _ := utils.Marshal(status)

	return ResourceStatus{
		ID:        id,
		Attribute: string(statusJSON),
	}
}
