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
	rconfig, _ := mesherykube.DetectKubeConfig()
	// Configure discovery client
	dclient, _ := discovery.NewClient(rconfig)

	namespaces, _ := dclient.ListNamespaces()
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
