package config

import corev1 "k8s.io/api/core/v1"

var LocalMeshsyncConfigMap corev1.ConfigMap = corev1.ConfigMap{
	// how it is defined in
	// https://github.com/meshery/meshery/blob/master/install/kubernetes/helm/meshery-operator/charts/meshery-meshsync/values.yaml#L8
	Data: map[string]string{
		"whitelist": `[{"Resource":"grafanas.v1beta1.grafana.integreatly.org","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"prometheuses.v1.monitoring.coreos.com","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"namespaces.v1.","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"configmaps.v1.","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"nodes.v1.","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"secrets.v1.","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"persistentvolumes.v1.","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"persistentvolumeclaims.v1.","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"replicasets.v1.apps","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"pods.v1.","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"services.v1.","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"deployments.v1.apps","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"statefulsets.v1.apps","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"daemonsets.v1.apps","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"ingresses.v1.networking.k8s.io","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"endpoints.v1.","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"endpointslices.v1.discovery.k8s.io","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"cronjobs.v1.batch","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"replicationcontrollers.v1.","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"storageclasses.v1.storage.k8s.io","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"clusterroles.v1.rbac.authorization.k8s.io","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"volumeattachments.v1.storage.k8s.io","Events":["ADDED","MODIFIED","DELETED"]},{"Resource":"apiservices.v1.apiregistration.k8s.io","Events":["ADDED","MODIFIED","DELETED"]}]`,
	},
}
