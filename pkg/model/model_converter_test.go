package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/meshery/meshkit/broker"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// newUnstructured builds an *unstructured.Unstructured from a nested map, the
// same shape the informer delivers to ParseList.
func newUnstructured(obj map[string]interface{}) unstructured.Unstructured {
	return unstructured.Unstructured{Object: obj}
}

// TestParseList_ManagedFieldsNotCaptured is the regression test for the payload
// bloat fix: metadata.managedFields must never be captured into the published
// resource, even when the source object carries it.
func TestParseList_ManagedFieldsNotCaptured(t *testing.T) {
	obj := newUnstructured(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "cm",
			"namespace": "default",
			"managedFields": []interface{}{
				map[string]interface{}{
					"manager":   "kube-controller-manager",
					"operation": "Update",
					"fieldsV1": map[string]interface{}{
						"f:data": map[string]interface{}{},
					},
				},
			},
		},
	})

	result := ParseList(obj, broker.Add, "cluster-1")

	if result.KubernetesResourceMeta == nil {
		t.Fatalf("expected metadata to be populated")
	}
	if result.KubernetesResourceMeta.ManagedFields != "" {
		t.Errorf("expected ManagedFields to be empty, got %q", result.KubernetesResourceMeta.ManagedFields)
	}
}

// TestParseList_LabelAndAnnotationValuesUnescaped asserts that label and
// annotation values are captured as decoded strings. Values that contain JSON
// escape sequences (quotes, newlines, backslashes), such as the
// kubectl.kubernetes.io/last-applied-configuration annotation, must not keep
// the escape sequences from the serialized object.
func TestParseList_LabelAndAnnotationValuesUnescaped(t *testing.T) {
	lastAppliedKey := "kubectl.kubernetes.io/last-applied-configuration"
	lastApplied := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm"}}` + "\n"
	description := "line one\nline \"two\" C:\\path é"

	obj := newUnstructured(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "cm",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app": "web",
			},
			"annotations": map[string]interface{}{
				lastAppliedKey: lastApplied,
				"description":  description,
			},
		},
	})

	result := ParseList(obj, broker.Add, "cluster-1")

	if result.KubernetesResourceMeta == nil {
		t.Fatalf("expected metadata to be populated")
	}

	labels := map[string]string{}
	for _, l := range result.KubernetesResourceMeta.Labels {
		labels[l.Key] = l.Value
	}
	if labels["app"] != "web" {
		t.Errorf("label app: got %q, want %q", labels["app"], "web")
	}

	annotations := map[string]string{}
	for _, a := range result.KubernetesResourceMeta.Annotations {
		annotations[a.Key] = a.Value
	}
	want := map[string]string{
		lastAppliedKey: lastApplied,
		"description":  description,
	}
	for key, value := range want {
		if annotations[key] != value {
			t.Errorf("annotation %s: got %q, want %q", key, annotations[key], value)
		}
	}
}

// TestParseList_SecretRedactionDisabledByDefault asserts the default (env var
// unset) behavior is byte-for-byte unchanged: Secret data/stringData passes
// through exactly as it appears in the source object.
func TestParseList_SecretRedactionDisabledByDefault(t *testing.T) {
	// Ensure the env var is unset for this test regardless of ambient state.
	t.Setenv(redactSecretsEnvKey, "")

	obj := newUnstructured(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      "db-creds",
			"namespace": "default",
		},
		"type": "Opaque",
		"data": map[string]interface{}{
			"password": "cGFzc3dvcmQ=",
			"username": "YWRtaW4=",
		},
		"stringData": map[string]interface{}{
			"token": "plaintext-token",
		},
	})

	result := ParseList(obj, broker.Add, "cluster-1")

	if result.Data == "" {
		t.Fatalf("expected Data to be populated")
	}
	assertJSONEqual(t, result.Data, map[string]string{
		"password": "cGFzc3dvcmQ=",
		"username": "YWRtaW4=",
	})
	assertJSONEqual(t, result.StringData, map[string]string{
		"token": "plaintext-token",
	})
}

// TestParseList_SecretRedactionEnabled asserts that when the opt-in env var is
// set, Secret data/stringData/binaryData VALUES are redacted while the KEYS are
// preserved.
func TestParseList_SecretRedactionEnabled(t *testing.T) {
	t.Setenv(redactSecretsEnvKey, "true")

	obj := newUnstructured(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      "db-creds",
			"namespace": "default",
		},
		"type": "Opaque",
		"data": map[string]interface{}{
			"password": "cGFzc3dvcmQ=",
			"username": "YWRtaW4=",
		},
		"stringData": map[string]interface{}{
			"token": "plaintext-token",
		},
		"binaryData": map[string]interface{}{
			"cert": "YmluYXJ5",
		},
	})

	result := ParseList(obj, broker.Add, "cluster-1")

	assertJSONEqual(t, result.Data, map[string]string{
		"password": redactedSecretPlaceholder,
		"username": redactedSecretPlaceholder,
	})
	assertJSONEqual(t, result.StringData, map[string]string{
		"token": redactedSecretPlaceholder,
	})
	assertJSONEqual(t, result.BinaryData, map[string]string{
		"cert": redactedSecretPlaceholder,
	})
}

// TestParseList_RedactionEnabledLeavesConfigMapUntouched asserts that redaction
// is scoped to Secret resources only: a ConfigMap's data is passed through even
// when redaction is enabled.
func TestParseList_RedactionEnabledLeavesConfigMapUntouched(t *testing.T) {
	t.Setenv(redactSecretsEnvKey, "true")

	obj := newUnstructured(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "app-config",
			"namespace": "default",
		},
		"data": map[string]interface{}{
			"log_level": "debug",
		},
	})

	result := ParseList(obj, broker.Add, "cluster-1")

	assertJSONEqual(t, result.Data, map[string]string{
		"log_level": "debug",
	})
}

func TestRedactSecretsEnabled(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"false", false},
		{"0", false},
		{"no", false},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv(redactSecretsEnvKey, tc.value)
			if got := redactSecretsEnabled(); got != tc.want {
				t.Errorf("redactSecretsEnabled() with %q = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestRedactSecretData(t *testing.T) {
	t.Run("redacts values preserving keys", func(t *testing.T) {
		out := redactSecretData([]byte(`{"password":"cGFzcw==","username":"YWRtaW4="}`))
		assertJSONEqual(t, out, map[string]string{
			"password": redactedSecretPlaceholder,
			"username": redactedSecretPlaceholder,
		})
	})

	t.Run("empty input is returned unchanged", func(t *testing.T) {
		if out := redactSecretData(nil); out != "" {
			t.Errorf("expected empty string, got %q", out)
		}
	})

	t.Run("malformed json is returned unchanged", func(t *testing.T) {
		in := []byte(`{not-json`)
		if out := redactSecretData(in); out != string(in) {
			t.Errorf("expected input returned unchanged, got %q", out)
		}
	})

	t.Run("empty object stays empty object", func(t *testing.T) {
		if out := redactSecretData([]byte(`{}`)); out != `{}` {
			t.Errorf("expected {}, got %q", out)
		}
	})
}

// TestParseList_NullSpecAndStatusDoesNotPanic verifies that resources carrying
// a JSON null spec or status do not cause a nil pointer panic in ParseList.
func TestParseList_NullSpecAndStatusDoesNotPanic(t *testing.T) {
	obj := newUnstructured(map[string]interface{}{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata": map[string]interface{}{
			"name":      "w",
			"namespace": "default",
		},
		"spec":   nil,
		"status": nil,
	})

	var result KubernetesResource
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseList panicked on null spec/status: %v", r)
			}
		}()
		result = ParseList(obj, broker.Add, "cluster-1")
	}()

	if result.Spec != nil {
		t.Errorf("expected Spec to stay nil for a null spec, got %+v", result.Spec)
	}
	if result.Status != nil {
		t.Errorf("expected Status to stay nil for a null status, got %+v", result.Status)
	}
}

// TestParseList_NonNullSpecAndStatusStillPopulated asserts the nil guards do
// not change behavior for regular objects: an object-valued spec/status is
// still captured into the Spec/Status attributes.
func TestParseList_NonNullSpecAndStatusStillPopulated(t *testing.T) {
	obj := newUnstructured(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "p",
			"namespace": "default",
		},
		"spec":   map[string]interface{}{"restartPolicy": "Always"},
		"status": map[string]interface{}{"phase": "Running"},
	})

	result := ParseList(obj, broker.Add, "cluster-1")

	if result.Spec == nil {
		t.Fatalf("expected Spec to be populated for an object-valued spec")
	}
	if !strings.Contains(result.Spec.Attribute, `"restartPolicy":"Always"`) {
		t.Errorf("unexpected Spec attribute: %q", result.Spec.Attribute)
	}
	if result.Status == nil {
		t.Fatalf("expected Status to be populated for an object-valued status")
	}
	if !strings.Contains(result.Status.Attribute, `"phase":"Running"`) {
		t.Errorf("unexpected Status attribute: %q", result.Status.Attribute)
	}
}

// assertJSONEqual compares a JSON string against an expected key/value map,
// tolerating key-ordering differences that json.Marshal does not guarantee.
func assertJSONEqual(t *testing.T, got string, want map[string]string) {
	t.Helper()

	var gotMap map[string]string
	if err := json.Unmarshal([]byte(got), &gotMap); err != nil {
		t.Fatalf("failed to unmarshal %q: %v", got, err)
	}
	if len(gotMap) != len(want) {
		t.Fatalf("map length mismatch: got %v, want %v", gotMap, want)
	}
	for k, v := range want {
		if gotMap[k] != v {
			t.Errorf("key %q: got %q, want %q", k, gotMap[k], v)
		}
	}
}
