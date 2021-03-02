package model

import (
	"fmt"

	"github.com/buger/jsonparser"
	"github.com/layer5io/meshkit/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ParseList(object unstructured.Unstructured) Object {
	data, _ := object.MarshalJSON()

	// ObjectMeta internal models
	labels := make([]*KeyValue, 0)
	annotations := make([]*KeyValue, 0)
	finalizers, _ := jsonparser.GetString(data, "metadata", "finalizers")
	managedFields, _ := jsonparser.GetString(data, "metadata", "managedFields")
	ownerReferences, _ := jsonparser.GetString(data, "metadata", "ownerReferences")
	_ = jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		labels = append(labels, &KeyValue{
			Key:   string(key),
			Value: string(value),
		})
		return nil
	}, "metadata", "labels")
	_ = jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		annotations = append(annotations, &KeyValue{
			Key:   string(key),
			Value: string(value),
		})
		return nil
	}, "metadata", "labels")

	result := Object{}
	_ = utils.Unmarshal(string(data), &result)

	result.ObjectMeta.Labels = labels
	result.ObjectMeta.Annotations = annotations
	result.ObjectMeta.Finalizers = finalizers
	result.ObjectMeta.ManagedFields = managedFields
	result.ObjectMeta.OwnerReferences = ownerReferences

	if spec, err := jsonparser.GetString(data, "spec"); err == nil {
		result.Spec.Attribute = spec
	}

	if status, err := jsonparser.GetString(data, "status"); err == nil {
		result.Status.Attribute = status
	}

	if immutable, err := jsonparser.GetString(data, "immutable"); err == nil {
		result.Immutable = immutable
	}

	if data, err := jsonparser.GetString(data, "data"); err == nil {
		result.Data = data
	}

	if binaryData, err := jsonparser.GetString(data, "binaryData"); err == nil {
		result.BinaryData = binaryData
	}

	if stringData, err := jsonparser.GetString(data, "stringData"); err == nil {
		result.StringData = stringData
	}

	if objType, err := jsonparser.GetString(data, "type"); err == nil {
		result.Type = objType
	}

	result.ResourceID = fmt.Sprintf("%s-%s-%s-%s", result.Kind, result.APIVersion, result.ObjectMeta.Namespace, result.ObjectMeta.Name)

	return result
}

func IsObject(obj Object) bool {
	if obj.ObjectMeta != nil && obj.Spec != nil && obj.Status != nil {
		return true
	}
	return false
}
