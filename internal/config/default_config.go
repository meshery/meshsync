package config

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

var (
	Server = map[string]string{
		"name":      "meshery-meshsync",
		"port":      "11000",
		"version":   "latest",
		"startedat": time.Now().String(),
	}

	Pipelines = map[string]PipelineConfigs{}

	Listeners = map[string]ListenerConfig{
		LogStream: {
			Name:           LogStream,
			ConnectionName: "meshsync-logstream",
			PublishTo:      "meshery.meshsync.logs",
		},
		ExecShell: {
			Name:           ExecShell,
			ConnectionName: "meshsync-exec",
			PublishTo:      "meshery.meshsync.exec",
		},
		RequestStream: {
			Name:           RequestStream,
			ConnectionName: "meshsync-request-stream",
			SubscribeTo:    "meshery.meshsync.request",
		},
	}

	DefaultEvents     = []string{"ADD", "UPDATE", "DELETE"}
	ExemptedResources = []string{"selfsubjectrulesreviews", "localsubjectaccessreviews", "bindings", "selfsubjectaccessreviews", "subjectaccessreviews", "tokenreviews", "componentstatuses", "flowschemas", "prioritylevelconfigurations"}
)

func PopulateDefaultResources(discoveryClient discovery.DiscoveryInterface) error {
	// Get all resources in the cluster
	clusterResources, namespacedResources, err := getAllResources(discoveryClient)
	if err != nil {
		fmt.Printf("Error getting all resources: %v\n", err)
		return ErrInitConfig(err)
	}
	var localResources []PipelineConfig

	for _, v := range namespacedResources {
		localResources = append(localResources, PipelineConfig{Name: v, Events: DefaultEvents, PublishTo: "meshery.meshsync.core"})
	}
	Pipelines[LocalResourceKey] = localResources

	var globalResources []PipelineConfig

	for _, v := range clusterResources {
		globalResources = append(globalResources, PipelineConfig{Name: v, Events: DefaultEvents, PublishTo: "meshery.meshsync.core"})
	}
	Pipelines[GlobalResourceKey] = globalResources

	return nil
}
func getAllResources(discoveryClient discovery.DiscoveryInterface) ([]string, []string, error) {

	_, groupList, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return nil, nil, err
	}

	var clusterResources []string
	var namespacedResources []string

	for _, group := range groupList {
		for _, resource := range group.APIResources {
			if strings.Contains(resource.Name, "/") {
				continue
			}
			groupVersion, _ := schema.ParseGroupVersion(group.GroupVersion)
			gvk := groupVersion.WithKind(resource.Kind)
			// gvk now contains the GroupVersionKind for the resource
			resStr := fmt.Sprintf("%s.%s.%s", resource.Name, gvk.Version, gvk.Group)

			// skip excempted resources
			if idx := slices.IndexFunc(ExemptedResources, func(c string) bool { return c == resource.Name }); idx != -1 {
				continue
			}

			//determine scope of the resource
			if resource.Namespaced {
				namespacedResources = append(namespacedResources, resStr)
			} else {
				clusterResources = append(clusterResources, resStr)
			}
		}
	}

	return clusterResources, namespacedResources, nil
}
