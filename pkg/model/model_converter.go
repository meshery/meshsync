package model

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshsync/internal/cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConvObject(typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta, spec interface{}, status interface{}) Object {
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
	_ = utils.Unmarshal(string(specJSON), &specTemp)

	return KubernetesResourceSpec{
		ResourceSpecID: id,
		Attribute:      specTemp,
	}
}

func getKubernetesResourceStatus(status interface{}, id string) KubernetesResourceStatus {
	statusJSON, _ := json.Marshal(status)
	var statusTemp map[string]interface{}
	_ = utils.Unmarshal(string(statusJSON), &statusTemp)

	return KubernetesResourceStatus{
		ResourceStatusID: id,
		Attribute:        statusTemp,
	}
}
