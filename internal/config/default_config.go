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

	DefaultPublishingSubject = "meshery.meshsync.core"

	Pipelines = map[string]PipelineConfigs{
		GlobalResourceKey: []PipelineConfig{
			// Core Resources
			{
				Name:      "namespaces.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "configmaps.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
			{
				Name:      "nodes.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
			{
				Name:      "secrets.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
			{
				Name:      "persistentvolumes.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
			{
				Name:      "persistentvolumeclaims.v1.",
				PublishTo: "meshery.meshsync.core",
				Events:    DefaultEvents,
			},
		},
		LocalResourceKey: []PipelineConfig{
			// Core Resources
			{
				Name:      "replicasets.v1.apps",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "pods.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "services.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "deployments.v1.apps",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "statefulsets.v1.apps",
				PublishTo: DefaultPublishingSubject,
			},
			{
				Name:      "daemonsets.v1.apps",
				PublishTo: DefaultPublishingSubject,
			},
			//Added Ingress support
			{
				Name:      "ingresses.v1.networking.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			// Added endpoint support
			{
				Name:      "endpoints.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			//Added endpointslice support
			{
				Name:      "endpointslices.v1.discovery.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			// Added cronJob support
			{
				Name:      "cronjobs.v1.batch",
				PublishTo: DefaultPublishingSubject,
			},
			//Added ReplicationController support
			{
				Name:      "replicationcontrollers.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			//Added storageClass support
			{
				Name:      "storageclasses.v1.storage.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added ClusterRole support
			{
				Name:      "clusterroles.v1.rbac.authorization.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added VolumeAttachment support
			{
				Name:      "volumeattachments.v1.storage.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added apiservice support
			{
				Name:      "apiservices.v1.apiregistration.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
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

	DefaultEvents = []string{"ADDED", "MODIFIED", "DELETED"}
)
