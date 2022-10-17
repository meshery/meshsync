package config

import (
	"time"
)

var (
	Server = map[string]string{
		"name":      "meshery-meshsync",
		"port":      "11000",
		"version":   "latest",
		"startedat": time.Now().String(),
	}

	Pipelines = map[string]PipelineConfigs{
		GlobalResourceKey: []PipelineConfig{
			// Core Resources
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
			{
				Name:      "persistentvolumes.v1.",
				PublishTo: "meshery.meshsync.core",
			},
			{
				Name:      "persistentvolumeclaims.v1.",
				PublishTo: "meshery.meshsync.core",
			},
		},
		LocalResourceKey: []PipelineConfig{
			// Core Resources
			{
				Name:      "replicasets.v1.apps",
				PublishTo: "meshery.meshsyc.core",
			},
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
			// Istio Resources
			// {
			// 	Name:      "virtualservices.v1beta1.networking.istio.io",
			// 	PublishTo: "meshery.meshsync.istio",
			// },
			// {
			// 	Name:      "gateways.v1beta1.networking.istio.io",
			// 	PublishTo: "meshery.meshsync.istio",
			// },
			// {
			// 	Name:      "destinationrules.v1beta1.networking.istio.io",
			// 	PublishTo: "meshery.meshsync.istio",
			// },
		},
	}

	Listeners = map[string]ListenerConfig{
		LogStream: {
			Name:           LogStream,
			ConnectionName: "meshsync-logstream",
			PublishTo:      "meshery.meshsync.logs",
		},
		ExecShell: {
			Name:           ExecShell,
			ConnectionName: "meshsync-exec",
			PublishTo:      "meshery.meshsync.exec",
		},
		RequestStream: {
			Name:           RequestStream,
			ConnectionName: "meshsync-request-stream",
			SubscribeTo:    "meshery.meshsync.request",
		},
	}
)
