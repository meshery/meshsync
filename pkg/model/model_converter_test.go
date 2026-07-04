package model

import (
	"encoding/json"
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
