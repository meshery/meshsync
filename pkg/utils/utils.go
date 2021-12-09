package utils

import (
	"context"

	"github.com/layer5io/meshkit/utils/kubernetes"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// clusterID is a unique identifier for the cluster
// in which MeshSync is running
var clusterID *string = nil

// GetClusterID returns a unique identifier for the cluster in which
// meshsync is running
//
// Notes:
// 1. If MeshSync is running out of cluster then the function will return
// an empty string
// 2. Function caches the cluster ID whenever it is invoked for the first time
// assuming that the cluster ID cannot and will not change throughout MeshSync's
// lifecycle
func GetClusterID() string {
	if clusterID != nil {
		return *clusterID
	}

	client, err := kubernetes.New(nil)
	if err != nil {
		return ""
	}

	ksns, err := client.KubeClient.CoreV1().Namespaces().Get(context.TODO(), "kube-system", v1.GetOptions{})
	if err != nil {
		return ""
	}

	uid := string(ksns.ObjectMeta.GetUID())
	clusterID = &uid

	return *clusterID
}
