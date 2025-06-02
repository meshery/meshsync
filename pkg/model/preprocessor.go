package model

import (
	"github.com/meshery/meshkit/broker"
)

type ProcessFunc interface {
	Process(obj []byte, k8sresource *KubernetesResource, evtype broker.EventType) error
}

func GetProcessorInstance(kind string) ProcessFunc {
	switch kind {
	case "Service":
		return &K8SService{}
	default:
		return nil
	}
}
