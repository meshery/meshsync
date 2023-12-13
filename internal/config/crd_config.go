package config

import (
	"context"
	"errors"

	"github.com/layer5io/meshkit/utils"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	namespace = "meshery"           // Namespace for the Custom Resource
	crName    = "meshery-meshsync"  // Name of the custom resource
	version   = "v1alpha1"          // Version of the Custom Resource
	group     = "meshery.layer5.io" //Group for the Custom Resource
	resource  = "meshsyncs"         //Name of the Resource
)

func AugmentDefaultResourcesWithCRD(dyClient dynamic.Interface) (*MeshsyncConfig, error) {
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

	configMap := corev1.ConfigMap{}
	err = utils.Unmarshal(string(configStr), &configMap)

	if err != nil {
		return nil, ErrInitConfig(err)
	}

	// populate the required configs
	meshsyncConfig, err := PopulateConfigs(configMap)

	if err != nil {
		return nil, ErrInitConfig(err)
	}
	return meshsyncConfig, nil
}

// PopulateConfigs compares the default configs and the whitelist and blacklist
func PopulateConfigs(configMap corev1.ConfigMap) (*MeshsyncConfig, error) {
	meshsyncConfig := &MeshsyncConfig{}

	if _, ok := configMap.Data["blacklist"]; ok {
		if len(configMap.Data["blacklist"]) > 0 {
			err := utils.Unmarshal(configMap.Data["blacklist"], &meshsyncConfig.BlackList)
			if err != nil {
				return nil, ErrInitConfig(err)
			}
		}
	}

	if _, ok := configMap.Data["whitelist"]; ok {
		if len(configMap.Data["whitelist"]) > 0 {
			err := utils.Unmarshal(configMap.Data["whitelist"], &meshsyncConfig.WhiteList)
			if err != nil {
				return nil, ErrInitConfig(err)
			}
		}
	}

	if len(meshsyncConfig.WhiteList) != 0 {
		for k, v := range Pipelines[GlobalResourceKey] {
			if idx := slices.IndexFunc(meshsyncConfig.WhiteList, func(c ResourceConfig) bool { return c.Resource == v.Name }); idx != -1 {
				config := meshsyncConfig.WhiteList[idx]
				Pipelines[GlobalResourceKey][k].Events = config.Events
			}
		}
		// Handle local resources
		for k, v := range Pipelines[LocalResourceKey] {
			if idx := slices.IndexFunc(meshsyncConfig.WhiteList, func(c ResourceConfig) bool { return c.Resource == v.Name }); idx != -1 {
				config := meshsyncConfig.WhiteList[idx]
				Pipelines[LocalResourceKey][k].Events = config.Events
			}
		}
	}
	if len(meshsyncConfig.BlackList) != 0 {

		for _, v := range Pipelines[GlobalResourceKey] {
			if idx := slices.IndexFunc(meshsyncConfig.BlackList, func(c string) bool { return c == v.Name }); idx != -1 {
				confIdx := slices.IndexFunc(Pipelines[GlobalResourceKey], func(c PipelineConfig) bool { return c.Name == v.Name })
				Pipelines[GlobalResourceKey] = slices.Delete(Pipelines[GlobalResourceKey], confIdx, confIdx+1)
			}
		}

		// Handle local resources
		for _, v := range Pipelines[LocalResourceKey] {
			if idx := slices.IndexFunc(meshsyncConfig.BlackList, func(c string) bool { return c == v.Name }); idx != -1 {
				confIdx := slices.IndexFunc(Pipelines[LocalResourceKey], func(c PipelineConfig) bool { return c.Name == v.Name })
				Pipelines[LocalResourceKey] = slices.Delete(Pipelines[LocalResourceKey], confIdx, confIdx+1)
			}
		}
	}

	return meshsyncConfig, nil
}
