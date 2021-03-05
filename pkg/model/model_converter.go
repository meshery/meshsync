package model

import (
	"encoding/base64"
	"fmt"

	"github.com/buger/jsonparser"
	"github.com/layer5io/meshkit/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ParseList(object unstructured.Unstructured) Object {
	data, _ := object.MarshalJSON()
	result := Object{}

	_ = utils.Unmarshal(string(data), &result)

	// ObjectMeta internal models
	labels := make([]*KeyValue, 0)
	_ = jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		labels = append(labels, &KeyValue{
			Key:   string(key),
			Value: string(value),
		})
		return nil
	}, "metadata", "labels")
	result.ObjectMeta.Labels = labels

	annotations := make([]*KeyValue, 0)
	_ = jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		annotations = append(annotations, &KeyValue{
			Key:   string(key),
			Value: string(value),
		})
		return nil
	}, "metadata", "labels")
	result.ObjectMeta.Annotations = annotations

	if finalizers, _, _, err := jsonparser.Get(data, "metadata", "finalizers"); err == nil {
		result.ObjectMeta.Finalizers = string(finalizers)
	}

	if managedFields, _, _, err := jsonparser.Get(data, "metadata", "managedFields"); err == nil {
		result.ObjectMeta.ManagedFields = string(managedFields)
	}

	if ownerReferences, _, _, err := jsonparser.Get(data, "metadata", "ownerReferences"); err == nil {
		result.ObjectMeta.OwnerReferences = string(ownerReferences)
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

	return result
}

func IsObject(obj Object) bool {
	return obj.ObjectMeta != nil
}

func SetID(obj *Object) {
	if obj != nil {
		id := base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("%s.%s.%s.%s", obj.Kind, obj.APIVersion, obj.ObjectMeta.Namespace, obj.ObjectMeta.Name),
		))
		obj.ID = id
		obj.ObjectMeta.ID = id

		if len(obj.ObjectMeta.Labels) > 0 {
			for _, label := range obj.ObjectMeta.Labels {
				label.ID = id
			}
		}

		if len(obj.ObjectMeta.Annotations) > 0 {
			for _, annotation := range obj.ObjectMeta.Annotations {
				annotation.ID = id
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
