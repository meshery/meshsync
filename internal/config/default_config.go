package config

import "time"

var (
	Server = map[string]string{
		"name":      "meshery-meshsync",
		"port":      "11000",
		"version":   "v0.0.1-alpha3",
		"startedat": time.Now().String(),
	}

	Pipelines = map[string]PipelineConfigs{
		GlobalResourceKey: []PipelineConfig{
			{
				Name:      "namespaces.v1.",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "configmaps.v1.",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "nodes.v1.",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "secrets.v1.",
				PublishTo: "meshery.meshsync.core",
			},
		},
		LocalResourceKey: []PipelineConfig{
			{
				Name:      "pods.v1.",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "services.v1.",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "deployments.v1.apps",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "statefulsets.v1.apps",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "daemonsets.v1.apps",
				PublishTo: "meshery.meshsync.core",
			},
		},
	}
)
