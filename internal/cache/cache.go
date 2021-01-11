package cache

import (
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
)

var (
	ClusterID  string
	Namespaces []string
)

func init() {
	Namespaces = make([]string, 0)
	ClusterID = uuid.New().String()
}

func SetNamespaces(namespaces []corev1.Namespace) {
	for _, namespace := range namespaces {
		Namespaces = append(Namespaces, namespace.Name)
	}
}
