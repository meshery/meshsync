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
			//Added 
			{
				Name:      "ingressclass.v1.networking.k8s.io",
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
			//Added container support
			{
				Name:	  "container.v1.core",
				PublishTo: DefaultPublishingSubject,
			},
			//Added job  support
			{
				Name:	  "job.v1.batch",
				PublishTo: DefaultPublishingSubject,
			},
			//Added service APIs support
			{
				Name:	  "service.apis",
				PublishTo: DefaultPublishingSubject,
			},
			//Added  csidriver support
			{
				Name:	  "csidriver.v1.storage.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added csinode  support
			{
				Name:	  "csinode.v1.storage.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added  csistoragecapacity support
			{
				Name:	  "csistoragecapacity.v1.storage.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added volume support
			{
				Name:	  "volume.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			//Added volumeattributesclass support
			{
				Name:	  "volumeattributesclass.v1alpha1.storage.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added clustertrustbundle support
			{
				Name:	  "clustertrustbundle.v1alpha1.certificates.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added  controllerversion support
			{
				Name:	  "controllerrevision.v1.apps",
				PublishTo: DefaultPublishingSubject,
			},
			//Added customresourcedefinition support
			{
				Name:	  "customresourcedefinition.v1.apiextensions.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added  event support
			{
				Name:	  "event.v1.events.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added limitrange support
			{
				Name:	  "limitrange.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			//Added horizontalpodautoscaler support
			{
				Name:	  "horizontalpodautoscaler.v2.autoscaling",
				PublishTo: DefaultPublishingSubject,
			},
			//Added mutatingwebhookconfiguration support
			{
				Name:	  "mutatingwebhookconfiguration.v1.admissionregistration.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added podschedulingcontext support
			{
				Name:	  "podschedulingcontext.v1alpha2.resource.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added podtemplate support
			{
				Name:	  "podtemplate.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			//Added poddistruptionbudget support
			{
				Name:	  "poddisruptionbudget.v1.policy",
				PublishTo: DefaultPublishingSubject,
			},
			//Added priorityclass support
			{
				Name:	  "priorityclass.v1.scheduling.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added resourceclaim support
			{
				Name:	  "resourceclaim.v1alpha2.resource.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added resourceclaimtemplate support
			{
				Name:	  "resourceclaimtemplate.v1alpha2.resource.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added resourceclass support
			{
				Name:	  "resourceclass.v1alpha2.resource.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added ValidatingWebhookConfiguration support
			{
				Name:	  "validatingwebhookconfiguration.v1.admissionregistration.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added ValidatingAdmissionPolicy support
			{
				Name:	  "validatingadmissionpolicy.v1beta1.admissionregistration.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added ValidatingAdmissionPolicyBinding support
			{
				Name:	  "validatingadmissionpolicybinding.v1beta1.admissionregistration.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added  binding support
			{
				Name:	  "binding.v1.",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added certificatesigningrequest support
			{
				Name:	  "certificatesigningrequest.v1.certificates.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},		
				
			//Added clusterrolebinding support
			{
				Name:	  "clusterrolebinding.v1.rbac.authorization.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added componentstatus support
			{
				Name:	  "componentstatus.v1.",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added flowschema support
			{
				Name:	  "flowschema.v1.flowcontrol.apiserver.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added IPAddress support
			{
				Name:	  "ipaddress.v1alpha1.networking.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added Lease support
			{
				Name:	  "lease.v1.coordination.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added LocalSubjectAccessReview support
			{
				Name:	  "localsubjectaccessreview.v1.authorization.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added Node support
			{
				Name:	  "node.v1.",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added NetworkPolicy support
			{
				Name:	  "networkpolicy.v1.networking.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added PriorityLevelConfiguration support
			{
				Name:	  "prioritylevelconfiguration.v1.flowcontrol.apiserver.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added resourcequota support
			{
				Name:	  "resourcequota.v1.",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added Role support
			{
				Name:	  "role.v1.rbac.authorization.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added RoleBinding support
			{
				Name:	  "rolebinding.v1.rbac.authorization.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added runtimeclass support
			{
				Name:	  "runtimeclass.v1.node.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added SelfSubjectAccessReview support
			{
				Name:	  "selfsubjectaccessreview.v1.authorization.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added SelfSubjectReview support
			{
				Name:	  "selfsubjectreview.v1.authentication.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},			
			//Added selfsubjectrulesreview support
			{
				Name:	  "selfsubjectrulesreview.v1.authorization.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added ServiceAccount support
			{
				Name:	  "serviceaccount.v1.",
				PublishTo: DefaultPublishingSubject,
			},
			//Added ServiceCIDR support
			{
				Name:	  "servicecidr.v1alpha1.networking.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added StorageVersion support
			{
				Name:	  "storageversion.v1alpha1.internal.apiserver.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added subjectaccessreview support
			{
				Name:	  "subjectaccessreview.v1.authorization.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added TokenRequest support
			{
				Name:	  "tokenrequest.v1.authentication.k8s.io",
				PublishTo: DefaultPublishingSubject,
			},
			//Added TokenReview support
			{
				Name:	  "tokenreview.v1.authentication.k8s.io",
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

	DefaultEvents = []string{"ADD", "UPDATE", "DELETE"}
)
