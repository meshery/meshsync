package model

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/buger/jsonparser"
	"github.com/google/uuid"
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/orchestration"
	iutils "github.com/layer5io/meshsync/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ParseList(object unstructured.Unstructured, eventType broker.EventType) KubernetesResource {
	data, _ := object.MarshalJSON()
	result := KubernetesResource{}
	_ = json.Unmarshal(data, &result)

	processorInstance := GetProcessorInstance(result.Kind)
	// ObjectMeta internal models
	labels := make([]*KubernetesKeyValue, 0)
	_ = jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		labels = append(labels, &KubernetesKeyValue{
			Kind:  KindLabel,
			Key:   string(key),
			Value: string(value),
		})

		if string(key) == orchestration.ResourceSourceDesignIdLabelKey {
			id, _ := uuid.FromBytes(value)
			result.PatternResource = &id
		}

		return nil
	}, "metadata", "labels")
	result.KubernetesResourceMeta.Labels = labels

	annotations := make([]*KubernetesKeyValue, 0)
	_ = jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		annotations = append(annotations, &KubernetesKeyValue{
			Kind:  KindAnnotation,
			Key:   string(key),
			Value: string(value),
		})
		return nil
	}, "metadata", "annotations")
	result.KubernetesResourceMeta.Annotations = annotations

	if finalizers, _, _, err := jsonparser.Get(data, "metadata", "finalizers"); err == nil {
		result.KubernetesResourceMeta.Finalizers = string(finalizers)
	}

	if managedFields, _, _, err := jsonparser.Get(data, "metadata", "managedFields"); err == nil {
		result.KubernetesResourceMeta.ManagedFields = string(managedFields)
	}

	if ownerReferences, _, _, err := jsonparser.Get(data, "metadata", "ownerReferences"); err == nil {
		result.KubernetesResourceMeta.OwnerReferences = string(ownerReferences)
	}

	if spec, _, _, err := jsonparser.Get(data, "spec"); err == nil {
		result.Spec.Attribute = string(spec)
	}

	if status, _, _, err := jsonparser.Get(data, "status"); err == nil {
		result.Status.Attribute = string(status)
	}

	if immutable, _, _, err := jsonparser.Get(data, "immutable"); err == nil {
		result.Immutable = string(immutable)
	}

	if objData, _, _, err := jsonparser.Get(data, "data"); err == nil {
		result.Data = string(objData)
	}

	if binaryData, _, _, err := jsonparser.Get(data, "binaryData"); err == nil {
		result.BinaryData = string(binaryData)
	}

	if stringData, _, _, err := jsonparser.Get(data, "stringData"); err == nil {
		result.StringData = string(stringData)
	}

	if objType, _, _, err := jsonparser.Get(data, "type"); err == nil {
		result.Type = string(objType)
	}

	result.ClusterID = iutils.GetClusterID()
	if processorInstance != nil {
		_ = processorInstance.Process(data, &result, eventType)
	}

	return result
}

func IsObject(obj KubernetesResource) bool {
	return obj.KubernetesResourceMeta != nil
}

func SetID(obj *KubernetesResource) {
	if obj != nil && IsObject(*obj) {
		id := base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("%s.%s.%s.%s.%s", obj.ClusterID, obj.Kind, obj.APIVersion, obj.KubernetesResourceMeta.Namespace, obj.KubernetesResourceMeta.Name),
		))
		obj.ID = id
		obj.KubernetesResourceMeta.ID = id

		if len(obj.KubernetesResourceMeta.Labels) > 0 {
			for _, label := range obj.KubernetesResourceMeta.Labels {
				label.ID = id
				label.UniqueID = uuid.New().String()
			}
		}

		if len(obj.KubernetesResourceMeta.Annotations) > 0 {
			for _, annotation := range obj.KubernetesResourceMeta.Annotations {
				annotation.ID = id
				annotation.UniqueID = uuid.New().String()
			}
		}

		if obj.Spec != nil {
			obj.Spec.ID = id
		}

		if obj.Status != nil {
			obj.Status.ID = id
		}
	}
}
