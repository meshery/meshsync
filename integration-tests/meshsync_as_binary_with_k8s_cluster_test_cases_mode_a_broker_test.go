package tests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/meshery/meshkit/broker"
	"github.com/stretchr/testify/assert"
)

var meshsyncBinaryWithK8SClusterBrokerModeTestsCasesData []meshsyncBinaryWithK8SClusterTestsCasesStruct = []meshsyncBinaryWithK8SClusterTestsCasesStruct{
	{
		name:            "output mode broker: number of messages received from broker is greater than zero",
		meshsyncCMDArgs: []string{"--stopAfter", "8s"},
		brokerMessageHandler: func(
			t *testing.T,
			out chan *broker.Message,
			resultData map[string]any,
		) {
			count := 0
			resultData["count"] = count
			go func() {
				for range out {
					count++
					resultData["count"] = count
				}
			}()
		},
		finalHandler: func(t *testing.T, resultData map[string]any) {
			count, ok := resultData["count"].(int)
			assert.True(t, ok, "must get count from result map")
			if ok {
				t.Logf("received %d messages from broker", count)
				assert.True(t, count > 0, "must receive messages from queue")
			}

		},
	},
	{
		name: "output mode broker: receive from broker only specified resources",
		meshsyncCMDArgs: []string{
			"--stopAfter",
			"8s",
			"--outputResources",
			"pod,replicaset",
		},
		brokerMessageHandler: func(
			t *testing.T,
			out chan *broker.Message,
			resultData map[string]any,
		) {
			resourcesCount := make(map[string]int)
			resultData["count"] = resourcesCount
			errCh := make(chan error)
			go func(errCh0 chan<- error) {
				for message := range out {
					k8sResource, err := unmarshalObject(message.Object)
					if err != nil {
						errCh0 <- fmt.Errorf(
							"error convering message.Object to model.KubernetesResource for %T",
							message.Object,
						)
						return
					}
					resourcesCount[strings.ToLower(k8sResource.Kind)]++
					resultData["count"] = resourcesCount
				}
			}(errCh)

			err := <-errCh
			if err != nil {
				t.Fatal(err)
			}
		},
		finalHandler: func(t *testing.T, resultData map[string]any) {
			count, ok := resultData["count"].(map[string]int)
			assert.True(t, ok, "must get count from result map")
			if ok {
				allowedKeys := map[string]bool{"pod": true, "replicaset": true}
				otherKeys := make([]string, 0)
				for k, v := range count {
					t.Logf("received %d messages from Kind %s", v, k)
					if !allowedKeys[k] {
						otherKeys = append(
							otherKeys,
							fmt.Sprintf("[%s = %v]", k, v),
						)
					}
				}
				assert.True(t, count["pod"] > 0, "must receive kind pod messages from queue")
				assert.True(t, count["replicaset"] > 0, "must receive kind replicaset messages from queue")
				if len(otherKeys) > 0 {
					t.Fatalf("received not allowed kind keys %s", strings.Join(otherKeys, ","))
				}
			}

		},
	},
	{
		name: "output mode broker: receive from broker only resources from specified namespace",
		meshsyncCMDArgs: []string{
			"--stopAfter",
			"8s",
			"--outputNamespace",
			"agile-otter",
		},
		brokerMessageHandler: func(
			t *testing.T,
			out chan *broker.Message,
			resultData map[string]any,
		) {
			resourcesPerNamespaceCount := make(map[string]int)
			resultData["count"] = resourcesPerNamespaceCount
			errCh := make(chan error)
			go func(errCh0 chan<- error) {
				for message := range out {
					k8sResource, err := unmarshalObject(message.Object)
					if err != nil {
						errCh0 <- fmt.Errorf(
							"error convering message.Object to model.KubernetesResource for %T",
							message.Object,
						)
						return
					}
					resourcesPerNamespaceCount[strings.ToLower(k8sResource.KubernetesResourceMeta.Namespace)]++
					resultData["count"] = resourcesPerNamespaceCount
				}
			}(errCh)

			err := <-errCh
			if err != nil {
				t.Fatal(err)
			}
		},
		finalHandler: func(t *testing.T, resultData map[string]any) {
			count, ok := resultData["count"].(map[string]int)
			assert.True(t, ok, "must get count from result map")
			if ok {
				allowedKeys := map[string]bool{"agile-otter": true}
				otherKeys := make([]string, 0)
				for k, v := range count {
					t.Logf("received %d messages for namespace %s", v, k)
					if !allowedKeys[k] {
						otherKeys = append(
							otherKeys,
							fmt.Sprintf("[%s = %v]", k, v),
						)
					}
				}
				assert.True(t, count["agile-otter"] > 0, "must receive messages from resources in agile-otter namespace")

				if len(otherKeys) > 0 {
					t.Fatalf("received not allowed namespace keys %s", strings.Join(otherKeys, ","))
				}
			}

		},
	},
	{
		name: "output mode broker: must not fail with a --broker-url param",
		meshsyncCMDArgs: []string{
			"--broker-url", "10.96.235.19:4222",
			"--stopAfter", "8s",
		},
		brokerMessageHandler: func(
			t *testing.T,
			out chan *broker.Message,
			resultData map[string]any,
		) {
			count := 0
			resultData["count"] = count
			go func() {
				for range out {
					count++
					resultData["count"] = count
				}
			}()
		},
		finalHandler: func(t *testing.T, resultData map[string]any) {
			count, ok := resultData["count"].(int)
			assert.True(t, ok, "must get count from result map")
			if ok {
				t.Logf("received %d messages from broker", count)
				assert.True(t, count > 0, "must receive messages from queue")
			}

		},
	},
}
