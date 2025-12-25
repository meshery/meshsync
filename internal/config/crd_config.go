package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/meshery/meshery-operator/pkg/client"
	"github.com/meshery/meshkit/utils"
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
	group     = "meshery.io"       // Group for the Custom Resource
	resource  = "meshsyncs"        // Name of the Resource
)

func GetMeshsyncCRDConfigs(dyClient dynamic.Interface) (*MeshsyncConfig, error) {
	// make a call to get the custom resource
	crd, err := GetMeshsyncCRD(dyClient)

	if err != nil {
		return nil, ErrInitConfig(err)
	}

	if crd == nil {
		return nil, ErrInitConfig(errors.New("custom Resource is nil"))
	}

	spec := crd.Object["spec"]
	specMap, ok := spec.(map[string]interface{})
	if !ok {
		return nil, ErrInitConfig(errors.New("unable to convert spec to map"))
	}
	configObj := specMap["watch-list"]
	if configObj == nil {
		return nil, ErrInitConfig(errors.New("custom Resource does not have Meshsync Configs"))
	}
	configStr, err := utils.Marshal(configObj)
	if err != nil {
		return nil, ErrInitConfig(err)
	}

	configMap := corev1.ConfigMap{}
	err = json.Unmarshal([]byte(configStr), &configMap)

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

func GetMeshsyncCRD(dyClient dynamic.Interface) (*unstructured.Unstructured, error) {
	// initialize the group version resource to access the custom resource
	gvr := schema.GroupVersionResource{Version: version, Group: group, Resource: resource}
	return dyClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), crName, metav1.GetOptions{})
}

func GetMeshsyncCRDConfigsLocal() (*MeshsyncConfig, error) {
	// populate the required configs
	meshsyncConfig, err := PopulateConfigsFromMap(LocalMeshsyncConfig)

	if err != nil {
		// // this hides actual error message
		// return nil, ErrInitConfig(err)
		return nil, err
	}
	return meshsyncConfig, nil
}

// PopulateConfigs compares the default configs and the whitelist and blacklist
func PopulateConfigs(configMap corev1.ConfigMap) (*MeshsyncConfig, error) {
	return PopulateConfigsFromMap(configMap.Data)
}

func PopulateConfigsFromMap(data map[string]string) (*MeshsyncConfig, error) {
	meshsyncConfig := &MeshsyncConfig{}

	// Populate whitelist and blacklist from input data
	if err := populateList(data, "blacklist", &meshsyncConfig.BlackList); err != nil {
		return nil, ErrInitConfig(err)
	}
	if err := populateList(data, "whitelist", &meshsyncConfig.WhiteList); err != nil {
		return nil, ErrInitConfig(err)
	}

	// Validate whitelist/blacklist
	if err := validateLists(meshsyncConfig); err != nil {
		return nil, ErrInitConfig(err)
	}

	// Populate pipelines based on whitelist/blacklist
	if len(meshsyncConfig.WhiteList) != 0 {
		populatePipelinesFromWhiteList(meshsyncConfig)
	} else {
		populatePipelinesFromBlackList(meshsyncConfig)
	}

	return meshsyncConfig, nil
}

// Populate either whitelist or blacklist from map
func populateList(data map[string]string, key string, target any) error {
	if val, ok := data[key]; ok && len(val) > 0 {
		if err := json.Unmarshal([]byte(val), target); err != nil {
			return err
		}
	}
	return nil
}

// Validate that exactly one of whitelist or blacklist is set
func validateLists(cfg *MeshsyncConfig) error {
	if len(cfg.BlackList) == 0 && len(cfg.WhiteList) == 0 {
		return errors.New("both whitelisted and blacklisted resources missing")
	}
	if len(cfg.BlackList) != 0 && len(cfg.WhiteList) != 0 {
		return errors.New("both whitelisted and blacklisted resources not currently supported")
	}
	return nil
}

// Populate pipelines for whitelist case
func populatePipelinesFromWhiteList(cfg *MeshsyncConfig) {
	globalPipelines := filterWhitelistedPipelines(Pipelines[GlobalResourceKey], cfg.WhiteList)
	localPipelines := filterWhitelistedPipelines(Pipelines[LocalResourceKey], cfg.WhiteList)

	if len(globalPipelines) > 0 || len(localPipelines) > 0 {
		cfg.Pipelines = make(map[string]PipelineConfigs)
	}
	if len(globalPipelines) > 0 {
		cfg.Pipelines[GlobalResourceKey] = globalPipelines
	}
	if len(localPipelines) > 0 {
		cfg.Pipelines[LocalResourceKey] = localPipelines
	}
}

// Filter pipelines based on whitelist
func filterWhitelistedPipelines(pipelines PipelineConfigs, whiteList []ResourceConfig) PipelineConfigs {
	result := make(PipelineConfigs, 0)
	for _, v := range pipelines {
		if idx := slices.IndexFunc(whiteList, func(c ResourceConfig) bool { return c.Resource == v.Name }); idx != -1 {
			config := whiteList[idx]
			v.Events = config.Events
			result = append(result, v)
		}
	}
	return result
}

// Populate pipelines for blacklist case
func populatePipelinesFromBlackList(cfg *MeshsyncConfig) {
	globalPipelines := filterBlacklistedPipelines(Pipelines[GlobalResourceKey], cfg.BlackList)
	localPipelines := filterBlacklistedPipelines(Pipelines[LocalResourceKey], cfg.BlackList)

	if len(globalPipelines) > 0 || len(localPipelines) > 0 {
		cfg.Pipelines = make(map[string]PipelineConfigs)
	}
	if len(globalPipelines) > 0 {
		cfg.Pipelines[GlobalResourceKey] = globalPipelines
	}
	if len(localPipelines) > 0 {
		cfg.Pipelines[LocalResourceKey] = localPipelines
	}
}

// Filter pipelines based on blacklist
func filterBlacklistedPipelines(pipelines PipelineConfigs, blackList []string) PipelineConfigs {
	result := make(PipelineConfigs, 0)
	for _, v := range pipelines {
		if idx := slices.IndexFunc(blackList, func(c string) bool { return c == v.Name }); idx == -1 {
			v.Events = DefaultEvents
			result = append(result, v)
		}
	}
	return result
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
