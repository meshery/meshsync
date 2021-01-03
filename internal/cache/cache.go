package cache

import (
	"github.com/google/uuid"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	discovery "github.com/layer5io/meshsync/pkg/discovery"
)

var (
	ClusterName string
	ClusterID   string
	Storage     map[string][]string
)

func init() {
	// Initialize Kubeconfig
	rconfig, err1 := mesherykube.DetectKubeConfig()
	if err1 != nil {
		return
	}

	// Configure discovery client
	dclient, err2 := discovery.NewClient(rconfig)
	if err2 != nil {
		return
	}

	namespaces, err3 := dclient.ListNamespaces()
	if err3 != nil {
		return
	}
	var namespacesName []string

	// processing
	for _, namespace := range namespaces {
		namespacesName = append(namespacesName, namespace.Name)
	}

	Storage = make(map[string][]string)
	Storage["NamespaceNames"] = namespacesName

	ClusterName = namespaces[0].ClusterName
	ClusterID = uuid.New().String()
}
