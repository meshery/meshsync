package utils

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// clusterID is a unique identifier for the cluster
// to which MeshSync has access to
var cachedMapOfClusterID = make(map[*kubernetes.Clientset]*string)

// GetClusterID returns a unique identifier for the cluster in which
// meshsync is running
//
// Notes:
// 1. If MeshSync is running out of cluster then the function will return
// an empty string
// 2. Function caches the cluster ID whenever it is invoked for the first time
// assuming that the cluster ID cannot and will not change throughout MeshSync's
// lifecycle
// 3. 2025-05-25:
// 3.1 If meshsync is running as library it could communicate with more than one cluster.
// 3.2 There is no more "in cluster", rather clusters to which meshsync have access to
// 3.3 cluster ID is still cached, but per kubeClient
func GetClusterID(kubeClient *kubernetes.Clientset) string {
	if cachedMapOfClusterID[kubeClient] != nil {
		return *cachedMapOfClusterID[kubeClient]
	}

	if kubeClient == nil {
		clusterID := ""
		cachedMapOfClusterID[kubeClient] = &clusterID
		return *cachedMapOfClusterID[kubeClient]
	}

	ksns, err := kubeClient.CoreV1().Namespaces().Get(context.TODO(), "kube-system", v1.GetOptions{})
	if err != nil {
		return ""
	}

	uid := string(ksns.ObjectMeta.GetUID())
	cachedMapOfClusterID[kubeClient] = &uid

	return *cachedMapOfClusterID[kubeClient]
}
