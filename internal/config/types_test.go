package config

import "testing"

func TestPipelineConfigsContains(t *testing.T) {
	p := PipelineConfigs{
		{Name: "certificates.v1.cert-manager.io"},
		{Name: "issuers.v1.cert-manager.io"},
	}

	if !p.Contains("issuers.v1.cert-manager.io") {
		t.Error("Contains should find an existing pipeline name")
	}
	if p.Contains("pods.v1.") {
		t.Error("Contains should not find an absent pipeline name")
	}
	if (PipelineConfigs{}).Contains("anything") {
		t.Error("empty PipelineConfigs should contain nothing")
	}
}
