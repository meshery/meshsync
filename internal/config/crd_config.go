package config

import (
	"context"
	"errors"

	"github.com/layer5io/meshkit/utils"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	namespace = "meshery"           // Namespace for the Custom Resource
	crName    = "meshery-meshsync"  // Name of the custom resource
	version   = "v1alpha1"          // Version of the Custom Resource
	group     = "meshery.layer5.io" //Group for the Custom Resource
	resource  = "meshsyncs"         //Name of the Resource
)

func GetMeshsyncCRDConfigs() (*MeshsyncConfig, error) {
	// Initialize kubeclient
	kubeClient, err := mesherykube.New(nil)
	if err != nil {
		return nil, ErrInitConfig(err)
	}
	// initialize the dynamic kube client
	dyClient := kubeClient.DynamicKubeClient

	// initialize the group version resource to access the custom resource
	gvr := schema.GroupVersionResource{Version: version, Group: group, Resource: resource}

	// make a call to get the custom resource
	crd, err := dyClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), crName, metav1.GetOptions{})

	if err != nil {
		return nil, ErrInitConfig(err)
	}

	if crd == nil {
		return nil, ErrInitConfig(errors.New("Custom Resource is nil"))
	}

	spec := crd.Object["spec"]
	specMap, ok := spec.(map[string]interface{})
	if !ok {
		return nil, ErrInitConfig(errors.New("Unable to convert spec to map"))
	}
	configObj := specMap["watch-list"]
	if configObj == nil {
		return nil, ErrInitConfig(errors.New("Custom Resource does not have Meshsync Configs"))
	}
	configStr, err := utils.Marshal(configObj)
	if err != nil {
		return nil, ErrInitConfig(err)
	}
	meshsyncConfig := MeshsyncConfig{}
	configMap := corev1.ConfigMap{}
	err = utils.Unmarshal(string(configStr), &configMap)

	if err != nil {
		return nil, ErrInitConfig(err)
	}

	if _, ok := configMap.Data["blacklist"]; ok {
		err = utils.Unmarshal(configMap.Data["blacklist"], &meshsyncConfig.BlackList)
		if err != nil {
			return nil, ErrInitConfig(err)
		}
	}

	if _, ok := configMap.Data["whitelist"]; ok {
		err = utils.Unmarshal(configMap.Data["whitelist"], &meshsyncConfig.WhiteList)
		if err != nil {
			return nil, ErrInitConfig(err)
		}
	}

	// Handle global resources
	globalPipelines := make(PipelineConfigs, 0)
	for _, v := range Pipelines[GlobalResourceKey] {
		if idx := slices.IndexFunc(meshsyncConfig.WhiteList, func(c ResourceConfig) bool { return c.Resource == v.Name }); idx != -1 {
			config := meshsyncConfig.WhiteList[idx]
			v.Events = config.Events
			globalPipelines = append(globalPipelines, v)
		}
	}
	if len(globalPipelines) > 0 {
		meshsyncConfig.Pipelines = map[string]PipelineConfigs{}
		meshsyncConfig.Pipelines[GlobalResourceKey] = globalPipelines
	}

	// Handle local resources
	localPipelines := make(PipelineConfigs, 0)
	for _, v := range Pipelines[LocalResourceKey] {
		if idx := slices.IndexFunc(meshsyncConfig.WhiteList, func(c ResourceConfig) bool { return c.Resource == v.Name }); idx != -1 {
			config := meshsyncConfig.WhiteList[idx]
			v.Events = config.Events
			localPipelines = append(localPipelines, v)
		}
	}

	if len(localPipelines) > 0 {
		if meshsyncConfig.Pipelines == nil {
			meshsyncConfig.Pipelines = make(map[string]PipelineConfigs)
		}
		meshsyncConfig.Pipelines[LocalResourceKey] = localPipelines
	}

	// Handle listeners
	listerners := make(ListenerConfigs, 0)
	for _, v := range Listeners {
		if idx := slices.IndexFunc(meshsyncConfig.WhiteList, func(c ResourceConfig) bool { return c.Resource == v.Name }); idx != -1 {
			config := meshsyncConfig.WhiteList[idx]
			v.Events = config.Events
			listerners = append(listerners, v)
		}
	}

	if len(listerners) > 0 {
		meshsyncConfig.Listeners = make(map[string]ListenerConfig)
		meshsyncConfig.Listeners = Listeners
	}

	return &meshsyncConfig, nil
}
