package utils

import (
	"context"
	"sync"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// cachedMapOfClusterID maps each kubeClient to the unique identifier of the
// cluster it reaches. MeshSync can run as a library once per cluster at the
// same time, so every access goes through cachedMapOfClusterIDMu.
var (
	cachedMapOfClusterIDMu sync.Mutex
	cachedMapOfClusterID   = make(map[*kubernetes.Clientset]*string)
)

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
	cachedMapOfClusterIDMu.Lock()
	cached := cachedMapOfClusterID[kubeClient]
	cachedMapOfClusterIDMu.Unlock()
	if cached != nil {
		return *cached
	}

	clusterID := ""
	if kubeClient != nil {
		// The API call runs without the lock, so a slow cluster does not block
		// the others; two concurrent calls for a new client may both fetch the UID.
		ksns, err := kubeClient.CoreV1().Namespaces().Get(context.TODO(), "kube-system", v1.GetOptions{})
		if err != nil {
			return ""
		}
		clusterID = string(ksns.GetUID())
	}

	cachedMapOfClusterIDMu.Lock()
	cachedMapOfClusterID[kubeClient] = &clusterID
	cachedMapOfClusterIDMu.Unlock()

	return clusterID
}
