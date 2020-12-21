package model

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/layer5io/meshsync/internal/cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type generalKubernetesResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              interface{} `json:"spec,omitempty"`
	Status            interface{} `json:"status,omitempty"`
}

func ConvInterface(obj interface{}) Object {
	jsonObj, _ := json.Marshal(obj)
	tempObject := generalKubernetesResource{}
	json.Unmarshal(jsonObj, &tempObject)

	return ConvModelObject(
		tempObject.TypeMeta,
		tempObject.ObjectMeta,
		tempObject.Spec,
		tempObject.Status,
	)
}

func ConvModelObject(typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta, spec interface{}, status interface{}) Object {
	kubernetesResource := getKubernetesResource()
	kubernetesResourceTypeMeta := getKubernetesResourceTypeMeta(typeMeta, kubernetesResource.ResourceTypeMetaID)
	kubernetesResourceObjectMeta := getKubernetesResourceObjectMeta(objectMeta, kubernetesResource.ResourceObjectMetaID)
	kubernetesResourceSpec := getKubernetesResourceSpec(spec, kubernetesResource.ResourceSpecID)
	kubernetesResourceStatus := getKubernetesResourceStatus(status, kubernetesResource.ResourceStatusID)

	return Object{
		Resource:   kubernetesResource,
		TypeMeta:   kubernetesResourceTypeMeta,
		ObjectMeta: kubernetesResourceObjectMeta,
		Spec:       kubernetesResourceSpec,
		Status:     kubernetesResourceStatus,
	}
}

func getKubernetesResource() KubernetesResource {
	return KubernetesResource{
		MesheryResourceID:    uuid.New().String(),
		ResourceID:           uuid.New().String(),
		ResourceTypeMetaID:   uuid.New().String(),
		ResourceObjectMetaID: uuid.New().String(),
		ResourceSpecID:       uuid.New().String(),
		ResourceStatusID:     uuid.New().String(),
	}
}

func getKubernetesResourceTypeMeta(resource metav1.TypeMeta, id string) KubernetesResourceTypeMeta {
	return KubernetesResourceTypeMeta{
		TypeMeta:           resource,
		ResourceTypeMetaID: id,
	}
}

func getKubernetesResourceObjectMeta(resource metav1.ObjectMeta, id string) KubernetesResourceObjectMeta {
	return KubernetesResourceObjectMeta{
		ObjectMeta:           resource,
		ResourceObjectMetaID: id,
		ClusterID:            cache.ClusterID,
	}
}

func getKubernetesResourceSpec(spec interface{}, id string) KubernetesResourceSpec {
	specJSON, _ := json.Marshal(spec)
	var specTemp map[string]interface{}
	json.Unmarshal(specJSON, &specTemp)

	return KubernetesResourceSpec{
		ResourceSpecID: id,
		Attribute:      specTemp,
	}
}

func getKubernetesResourceStatus(status interface{}, id string) KubernetesResourceStatus {
	statusJSON, _ := json.Marshal(status)
	var statusTemp map[string]interface{}
	json.Unmarshal(statusJSON, &statusTemp)

	return KubernetesResourceStatus{
		ResourceStatusID: id,
		Attribute:        statusTemp,
	}
}
