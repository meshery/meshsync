package cluster

import (
	broker "github.com/layer5io/meshsync/pkg/broker"
	discovery "github.com/layer5io/meshsync/pkg/discovery"
	inf "github.com/layer5io/meshsync/pkg/informers"
	informers "github.com/layer5io/meshsync/pkg/cluster/informers"
	pipeline "github.com/layer5io/meshsync/pkg/cluster/pipeline"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type Resources struct {
	Global GlobalResources `json:"global,omitempty"`
	Local  LocalResources  `json:"local,omitempty"`
}

type GlobalResources struct {
	Nodes      []corev1.Node      `json:"nodes,omitempty"`
	Namespaces []corev1.Namespace `json:"namespaces,omitempty"`
}

type LocalResources struct {
	Deployments []appsv1.Deployment `json:"deployments,omitempty"`
	Services []corev1.Service `json:"Services,omitempty"`
	Pods        []corev1.Pod        `json:"pods,omitempty"`
}

func Setup(dclient *discovery.Client, broker broker.Handler, iclient *inf.Client) error {
	// Get pipeline instance
	pl := pipeline.Initialize(dclient, broker)
	if pl.Run().Error != nil {
		return ErrInitPipeline(pl.Run().Error)
	}

	err := informers.Initialize(iclient, broker)
	if err != nil {
		return ErrInitInformer(err)
	}

	return nil
}
