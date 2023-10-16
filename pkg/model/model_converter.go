package model

import (
	"encoding/base64"
	"fmt"

	"github.com/buger/jsonparser"
	"github.com/google/uuid"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshsync/internal/config"
	iutils "github.com/layer5io/meshsync/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ParseList(object unstructured.Unstructured) KubernetesObject {
	data, _ := object.MarshalJSON()
	result := KubernetesObject{}

	_ = utils.Unmarshal(string(data), &result)

	// ObjectMeta internal models
	labels := make([]*KubernetesKeyValue, 0)
	_ = jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		labels = append(labels, &KubernetesKeyValue{
			Kind:  KindLabel,
			Key:   string(key),
			Value: string(value),
		})

		if string(key) == config.PatternResourceIDLabelKey {
			id, _ := uuid.FromBytes(value)
			result.PatternResource = &id
		}

		return nil
	}, "metadata", "labels")
	result.KubernetesObjectMeta.Labels = labels

	annotations := make([]*KubernetesKeyValue, 0)
	_ = jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		annotations = append(annotations, &KubernetesKeyValue{
			Kind:  KindAnnotation,
			Key:   string(key),
			Value: string(value),
		})
		return nil
	}, "metadata", "annotations")
	result.KubernetesObjectMeta.Annotations = annotations

	if finalizers, _, _, err := jsonparser.Get(data, "metadata", "finalizers"); err == nil {
		result.KubernetesObjectMeta.Finalizers = string(finalizers)
	}

	if managedFields, _, _, err := jsonparser.Get(data, "metadata", "managedFields"); err == nil {
		result.KubernetesObjectMeta.ManagedFields = string(managedFields)
	}

	if ownerReferences, _, _, err := jsonparser.Get(data, "metadata", "ownerReferences"); err == nil {
		result.KubernetesObjectMeta.OwnerReferences = string(ownerReferences)
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

	return result
}

func IsObject(obj KubernetesObject) bool {
	return obj.KubernetesObjectMeta != nil
}

func SetID(obj *KubernetesObject) {
	if obj != nil && IsObject(*obj) {
		id := base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("%s.%s.%s.%s.%s", obj.ClusterID, obj.Kind, obj.APIVersion, obj.KubernetesObjectMeta.Namespace, obj.KubernetesObjectMeta.Name),
		))
		obj.ID = id
		obj.KubernetesObjectMeta.ID = id

		if len(obj.KubernetesObjectMeta.Labels) > 0 {
			for _, label := range obj.KubernetesObjectMeta.Labels {
				label.ID = id
				label.UniqueID = uuid.New().String()
			}
		}

		if len(obj.KubernetesObjectMeta.Annotations) > 0 {
			for _, annotation := range obj.KubernetesObjectMeta.Annotations {
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
