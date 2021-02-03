package model

import (
	"fmt"

	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshsync/internal/cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConvObject(typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta, spec interface{}, status interface{}) Object {
	resourceIdentifier := fmt.Sprintf("%s-%s-%s-%s", typeMeta.Kind, typeMeta.APIVersion, objectMeta.Namespace, objectMeta.Name)
	resourceTypeMeta := makeTypeMeta(typeMeta)
	resourceObjectMeta := makeObjectMeta(objectMeta)
	resourceSpec := makeSpec(spec)
	resourceStatus := makeStatus(status)

	return Object{
		ResourceID: resourceIdentifier,
		TypeMeta:   resourceTypeMeta,
		ObjectMeta: resourceObjectMeta,
		Spec:       resourceSpec,
		Status:     resourceStatus,
	}
}

func makeTypeMeta(resource metav1.TypeMeta) *ResourceTypeMeta {
	return &ResourceTypeMeta{
		Kind:       resource.Kind,
		APIVersion: resource.APIVersion,
	}
}

func makeObjectMeta(resource metav1.ObjectMeta) *ResourceObjectMeta {
	var creationTime string
	var deletionTime string
	if !resource.CreationTimestamp.IsZero() {
		creationTime = resource.CreationTimestamp.String()
	}
	if !resource.DeletionTimestamp.IsZero() {
		deletionTime = resource.DeletionTimestamp.String()
	}

	return &ResourceObjectMeta{
		Name:                       resource.Name,
		GenerateName:               resource.GenerateName,
		Namespace:                  resource.Namespace,
		SelfLink:                   resource.SelfLink,
		UID:                        string(resource.UID),
		ResourceVersion:            resource.ResourceVersion,
		CreationTimestamp:          creationTime,
		DeletionTimestamp:          deletionTime,
		Labels:                     makeLabelsOrAnnotations(resource.Labels),
		Annotations:                makeLabelsOrAnnotations(resource.Annotations),
		Generation:                 resource.Generation,
		DeletionGracePeriodSeconds: resource.DeletionGracePeriodSeconds,
		// OwnerReferences:            resource.OwnerReferences,
		// Finalizers:  resource.Finalizers,
		ClusterName: resource.ClusterName,
		// ManagedFields:              resource.ManagedFields,
		ClusterID: cache.ClusterID,
	}
}

func makeSpec(spec interface{}) *ResourceSpec {
	specJSON, _ := utils.Marshal(spec)

	return &ResourceSpec{
		Attribute: string(specJSON),
	}
}

func makeStatus(status interface{}) *ResourceStatus {
	statusJSON, _ := utils.Marshal(status)

	return &ResourceStatus{
		Attribute: string(statusJSON),
	}
}

func makeLabelsOrAnnotations(items map[string]string) []*KeyValue {
	result := make([]*KeyValue, 0)
	for key, val := range items {
		result = append(result, &KeyValue{
			Key:   key,
			Value: val,
		})
	}
	return result
}
