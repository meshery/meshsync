package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/layer5io/meshery-operator/pkg/client"
	"github.com/layer5io/meshkit/utils"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var (
	namespace = "meshery"          // Namespace for the Custom Resource
	crName    = "meshery-meshsync" // Name of the custom resource
	version   = "v1alpha1"         // Version of the Custom Resource
	group     = "meshery.io"       //Group for the Custom Resource
	resource  = "meshsyncs"        //Name of the Resource
)

// ValidateMeshsyncCRD validates the MeshSync CRD structure and required fields
func ValidateMeshsyncCRD(crd map[string]interface{}) error {
	// Validate spec exists
	spec, exists := crd["spec"].(map[string]interface{})
	if !exists {
		return ErrInitConfig(errors.New("invalid CRD: spec field is missing"))
	}

	// Validate watch-list exists in spec
	_, exists = spec["watch-list"]
	if !exists {
		return ErrInitConfig(errors.New("invalid CRD: watch-list field is missing in spec"))
	}

	// Validate version exists in spec
	_, exists = spec["version"]
	if !exists {
		return ErrInitConfig(errors.New("invalid CRD: version field is missing in spec"))
	}
	
	return nil
}


func GetMeshsyncCRDConfigs(dyClient dynamic.Interface) (*MeshsyncConfig, error) {
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

	// Validate CRD structure
	if err := ValidateMeshsyncCRD(crd.Object); err != nil {
		return nil, err
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

	// ensure that atleast one of whitelist or blacklist has been supplied
	if len(meshsyncConfig.BlackList) == 0 && len(meshsyncConfig.WhiteList) == 0 {
		return nil, ErrInitConfig(errors.New("Both whitelisted and blacklisted resources missing"))
	}

	// ensure that only one of whitelist or blacklist has been supplied
	if len(meshsyncConfig.BlackList) != 0 && len(meshsyncConfig.WhiteList) != 0 {
		return nil, ErrInitConfig(errors.New("Both whitelisted and blacklisted resources not currently supported"))
	}

	// Handle global resources
	globalPipelines := make(PipelineConfigs, 0)
	localPipelines := make(PipelineConfigs, 0)

	if len(meshsyncConfig.WhiteList) != 0 {
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

	} else {

		for _, v := range Pipelines[GlobalResourceKey] {
			if idx := slices.IndexFunc(meshsyncConfig.BlackList, func(c string) bool { return c == v.Name }); idx == -1 {
				v.Events = DefaultEvents
				globalPipelines = append(globalPipelines, v)
			}
		}
		if len(globalPipelines) > 0 {
			meshsyncConfig.Pipelines = map[string]PipelineConfigs{}
			meshsyncConfig.Pipelines[GlobalResourceKey] = globalPipelines
		}

		// Handle local resources
		for _, v := range Pipelines[LocalResourceKey] {
			if idx := slices.IndexFunc(meshsyncConfig.BlackList, func(c string) bool { return c == v.Name }); idx == -1 {
				v.Events = DefaultEvents
				localPipelines = append(localPipelines, v)
			}
		}

		if len(localPipelines) > 0 {
			if meshsyncConfig.Pipelines == nil {
				meshsyncConfig.Pipelines = make(map[string]PipelineConfigs)
			}
			meshsyncConfig.Pipelines[LocalResourceKey] = localPipelines
		}
	}

	return meshsyncConfig, nil
}

func PatchCRVersion(config *rest.Config) error {
	meshsyncClient, err := client.New(config)
	if err != nil {
		return ErrInitConfig(fmt.Errorf("unable to update MeshSync configuration"))
	}

	patchedResource := map[string]interface{}{
		"spec": map[string]interface{}{
			"version": Server["version"],
		},
	}
	byt, err := utils.Marshal(patchedResource)
	if err != nil {
		return ErrInitConfig(fmt.Errorf("unable to update MeshSync configuration"))
	}
	_, err = meshsyncClient.CoreV1Alpha1().MeshSyncs("meshery").Patch(context.TODO(), crName, types.MergePatchType, []byte(byt), metav1.PatchOptions{})
	if err != nil {
		return ErrInitConfig(fmt.Errorf("unable to update MeshSync configuration"))
	}
	return nil
}

// InitializeDefaultConfig creates a default configuration if none exists
func InitializeDefaultConfig(dyClient dynamic.Interface) error {
	gvr := schema.GroupVersionResource{Version: version, Group: group, Resource: resource}
	
	// Check if CR already exists
	_, err := dyClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), crName, metav1.GetOptions{})
	if err == nil {
		// CR already exists, no initialization needed
		return nil
	}
	
	// Create default configuration
	defaultCR := map[string]interface{}{
		"apiVersion": fmt.Sprintf("%s/%s", group, version),
		"kind": "MeshSync",
		"metadata": map[string]interface{}{
			"name": crName,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"version": Server["version"],
			"watch-list": GenerateDefaultWatchList(),
		},
	}
	
	// Convert to unstructured
	obj, err := utils.Marshal(defaultCR)
	if err != nil {
		return ErrInitConfig(fmt.Errorf("failed to marshal default config: %w", err))
	}
	
	var unstructuredObj map[string]interface{}
	if err := utils.Unmarshal(string(obj), &unstructuredObj); err != nil {
		return ErrInitConfig(fmt.Errorf("failed to unmarshal default config: %w", err))
	}
	
	// Create the CR
	_, err = dyClient.Resource(gvr).Namespace(namespace).Create(
		context.TODO(),
		&unstructured.Unstructured{Object: unstructuredObj},
		metav1.CreateOptions{},
	)
	if err != nil {
		return ErrInitConfig(fmt.Errorf("failed to create default CR: %w", err))
	}
	
	return nil
}

// GenerateDefaultWatchList creates a default watch list configuration
func GenerateDefaultWatchList() map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind": "ConfigMap",
		"data": map[string]string{
			"include-namespaces": "default,kube-system,meshery",
			"exclude-namespaces": "",
			"default-resources": "true",
			"include-resources": "",
			"exclude-resources": "",
		},
	}
}

