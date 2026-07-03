package model

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/google/uuid"
	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/orchestration"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// redactSecretsEnvKey is the environment variable that opts MeshSync into
// redacting Kubernetes Secret contents before they are published over the
// broker. It is OFF by default to preserve the existing cross-repo contract:
// Meshery Server may consume Secret data (data/stringData/binaryData) that is
// shipped over the broker, so changing the default could break downstream
// behavior. When set to a truthy value ("true"/"1"), Secret values are replaced
// with redactedSecretPlaceholder while the keys are preserved, so operators can
// still see which Secret keys exist without the plaintext/base64 payloads
// leaving the cluster.
const redactSecretsEnvKey = "MESHSYNC_REDACT_SECRETS"

// redactedSecretPlaceholder replaces Secret values when redaction is enabled.
const redactedSecretPlaceholder = "[REDACTED]"

// secretKind is the Kubernetes Kind for which Secret redaction applies.
const secretKind = "Secret"

// redactSecretsEnabled reports whether Secret redaction has been opted into via
// the MESHSYNC_REDACT_SECRETS environment variable. It mirrors the truthy
// parsing used elsewhere in MeshSync (see DEBUG handling in main.go).
func redactSecretsEnabled() bool {
	v := os.Getenv(redactSecretsEnvKey)
	return strings.ToLower(v) == "true" || v == "1"
}

// redactSecretData takes the raw JSON object captured from a Secret's
// data/stringData/binaryData field (e.g. {"password":"cGFzcw=="}) and returns
// an equivalent JSON object with every value replaced by
// redactedSecretPlaceholder while preserving the keys. If the input is not a
// JSON object (empty, malformed, or unexpected shape), it is returned
// unchanged so redaction never corrupts or drops data it does not understand.
func redactSecretData(raw string) string {
	if raw == "" {
		return raw
	}

	var kv map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &kv); err != nil {
		return raw
	}

	redacted := make(map[string]string, len(kv))
	for key := range kv {
		redacted[key] = redactedSecretPlaceholder
	}

	out, err := json.Marshal(redacted)
	if err != nil {
		return raw
	}
	return string(out)
}

// TODO fix cyclop error
// Error: pkg/model/model_converter.go:16:1: calculated cyclomatic complexity for function ParseList is 13, max is 10 (cyclop)
//
//nolint:cyclop
func ParseList(
	object unstructured.Unstructured,
	eventType broker.EventType,
	clusterID string,
) KubernetesResource {
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

	// metadata.managedFields is intentionally NOT captured. It is large,
	// high-churn server-side-apply bookkeeping that is never persisted
	// (the ManagedFields struct field is gorm:"-"), yet it was previously
	// serialized and published over the broker on every event, inflating
	// every message. The ManagedFields struct field is left in place
	// (unpopulated) because KubernetesResourceObjectMeta is a shared contract
	// consumed by meshery-server; removing the field would be a breaking change.

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

	// Secret contents (data/stringData/binaryData) are captured and shipped
	// over the broker. Meshery Server may consume this data, so the default
	// behavior is unchanged (values pass through as-is). When an operator opts
	// in via MESHSYNC_REDACT_SECRETS, Secret values are redacted here while the
	// keys are preserved, keeping plaintext/base64 secrets from leaving the
	// cluster. Redaction applies to Secret resources only; ConfigMap data is
	// left untouched.
	redactSecrets := redactSecretsEnabled() && result.Kind == secretKind

	if objData, _, _, err := jsonparser.Get(data, "data"); err == nil {
		result.Data = string(objData)
		if redactSecrets {
			result.Data = redactSecretData(result.Data)
		}
	}

	if binaryData, _, _, err := jsonparser.Get(data, "binaryData"); err == nil {
		result.BinaryData = string(binaryData)
		if redactSecrets {
			result.BinaryData = redactSecretData(result.BinaryData)
		}
	}

	if stringData, _, _, err := jsonparser.Get(data, "stringData"); err == nil {
		result.StringData = string(stringData)
		if redactSecrets {
			result.StringData = redactSecretData(result.StringData)
		}
	}

	if objType, _, _, err := jsonparser.Get(data, "type"); err == nil {
		result.Type = string(objType)
	}

	result.ClusterID = clusterID
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
