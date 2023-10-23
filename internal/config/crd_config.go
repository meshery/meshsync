package config

import (
	"context"
	"errors"

	"github.com/layer5io/meshkit/utils"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
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
	configObj := spec.(map[string]interface{})["config"]
	specMap, ok := spec.(map[string]interface{})
	if !ok {
		return nil, ErrInitConfig(errors.New("Unable to convert spec to map"))
	}
	configObj := specMap["config"]
	if configObj == nil {
		return nil, ErrInitConfig(errors.New("Custom Resource does not have Meshsync Configs"))
	}
	configStr, err := utils.Marshal(configObj)
	if err != nil {
		return nil, ErrInitConfig(err)
	}
	meshsyncConfig := MeshsyncConfig{}
	err = utils.Unmarshal(string(configStr), &meshsyncConfig)

	if err != nil {
		return nil, ErrInitConfig(err)
	}
	return &meshsyncConfig, nil
}
