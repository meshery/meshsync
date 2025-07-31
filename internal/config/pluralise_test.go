package config

import "testing"

func TestPluralize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"pod", "pods"},
		{"ingress", "ingresses"},
		{"replicaset", "replicasets"},
		{"configmap", "configmaps"},
		{"job", "jobs"},
		{"endpoint", "endpoints"},
		{"cronjob", "cronjobs"},
		{"customresourcedefinition", "customresourcedefinitions"},
		{"storageclass", "storageclasses"},
		{"clusterrole", "clusterroles"},
		{"box", "boxes"},
		{"buzz", "buzzes"},
		{"church", "churches"},
		{"bush", "bushes"},
		{"cat", "cats"},
		{"bus", "buses"},
		{"fox", "foxes"},
		{"quiz", "quizes"}, // with respect to English it is not plural, but it fits in our logic
	}

	for _, test := range tests {
		got := pluralize(test.input)
		if got != test.expected {
			t.Errorf("pluralize(%q) = %q; want %q", test.input, got, test.expected)
		}
	}
}
