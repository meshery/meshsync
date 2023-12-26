package model

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/kubernetes"
	v1 "k8s.io/api/core/v1"
)

type K8SService struct{}

func (s *K8SService) Process(data []byte, k8sresource *KubernetesResource, evtype broker.EventType) error {
	if evtype == broker.Delete {
		return nil
	}

	k8sservice := &v1.Service{}

	err := utils.Unmarshal(string(data), k8sservice)
	if err != nil {
		return err
	}

	urls := []string{}
	endpoint, err := kubernetes.GetEndpoint(context.Background(), &kubernetes.ServiceOptions{}, k8sservice)
	if err != nil {
		return err
	}

	if endpoint != nil {
		if endpoint.External != nil {
			url, err := s.validateURL(endpoint.External.Address, endpoint.External.Port)
			if err == nil {
				urls = append(urls, url)
			}
		}
		if endpoint.Internal != nil {
			url, err := s.validateURL(endpoint.Internal.Address, endpoint.Internal.Port)
			if err == nil {
				urls = append(urls, url)
			}
		}
	}

	if k8sresource.ComponentMetadata == nil {
		k8sresource.ComponentMetadata = make(map[string]interface{})
	}
	k8sresource.ComponentMetadata = map[string]interface{}{
		"capabilities": map[string]interface{}{
                        // indicates that this svc can be upgraded to "Meshery Connection".
			"connection": true,
			"urls":       urls,
		},
	}

	return nil
}

func (s *K8SService) validateURL(address string, port int32) (serviceurl string, err error) {
	protocol := "http"
	if port == 443 {
		protocol = "https"
	}

	// For some Cluster IP type svc the address is set as None
	// Hence to prevent adding these IPs as URLs, below check is added.
	if strings.Contains(strings.ToLower(address), "none") {
		return serviceurl, kubernetes.ErrEndpointNotFound
	}
	serviceurl = fmt.Sprintf("%s://%s:%d", protocol, address, port)
	_, err = url.ParseRequestURI(serviceurl)
	return serviceurl, err
}
