package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/pkg/model"
	"github.com/stretchr/testify/assert"
)

const (
	resyncSuccessThreshold        = 5
	minExpectedObjectsAfterResync = 3
)

var meshsyncBinaryWithK8SClusterBrokerModeTestsCasesData []meshsyncBinaryWithK8SClusterTestsCasesStruct = []meshsyncBinaryWithK8SClusterTestsCasesStruct{
	{
		name:            "output mode broker: number of messages received from broker is greater than zero",
		meshsyncCMDArgs: []string{"--stopAfter", "8s"},
		brokerMessageHandler: func(
			t *testing.T,
			br broker.Handler,
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
			br broker.Handler,
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
			"--outputNamespaces",
			"agile-otter",
		},
		brokerMessageHandler: func(
			t *testing.T,
			br broker.Handler,
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
			br broker.Handler,
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
		name:            "meshsync handles ReSync request and republishes cluster data",
		meshsyncCMDArgs: []string{"--stopAfter", "20s"},

		brokerMessageHandler: func(t *testing.T, br broker.Handler, out chan *broker.Message, resultData map[string]any) {
			t.Helper()

			beforeResync := make(map[string]bool)
			afterResync := make(map[string]bool)

			// Drain initial data from meshsync
			t.Log("Draining initial meshsync discovery...")

			idleTimer := time.NewTimer(3 * time.Second)

		DrainLoop:
			for {
				select {
				case msg, ok := <-out:
					if !ok {
						break DrainLoop
					}

					if !idleTimer.Stop() {
						<-idleTimer.C
					}
					idleTimer.Reset(3 * time.Second)

					if msg.Object != nil {
						kr, err := unmarshalObject(msg.Object)
						if err == nil && kr.Kind != "" {
							key := kr.Kind + ":" + kr.KubernetesResourceMeta.Name
							beforeResync[key] = true
						}
					}

				case <-idleTimer.C:
					break DrainLoop
				}
			}

			t.Logf("Initial discovery completed. %d unique objects stored", len(beforeResync))

			// Trigger ReSync
			err := br.Publish("meshery.meshsync.request", &broker.Message{
				Request: &broker.RequestObject{
					Entity: broker.RequestEntity("resync-discovery"),
				},
			})
			if err != nil {
				t.Fatalf("failed to publish ReSync request: %v", err)
			}

			t.Log("ReSync request sent, listening for new objects...")

			// Capture re-discovered objects
			resyncCount := 0
			timeout := time.After(15 * time.Second)

			for {
				select {
				case msg, ok := <-out:
					if !ok {
						goto END
					}

					if msg.Object != nil {
						kr, err := unmarshalObject(msg.Object)
						if err == nil && kr.Kind != "" {
							key := kr.Kind + ":" + kr.KubernetesResourceMeta.Name
							afterResync[key] = true
							resultData["last_object"] = kr
							resyncCount++
						}
					}

					if resyncCount >= resyncSuccessThreshold {
						goto END
					}

				case <-timeout:
					t.Logf("Timeout reached, received %d objects after resync", resyncCount)
					goto END
				}
			}

		END:
			// Compare before & after (INTELLIGENT CHECK)
			matchCount := 0
			for k := range beforeResync {
				if afterResync[k] {
					matchCount++
				}
			}

			resultData["count"] = resyncCount
			resultData["matches"] = matchCount
			resultData["received"] = resyncCount > 0

			t.Logf("ReSync completed — received: %d, matched: %d", resyncCount, matchCount)
		},

		finalHandler: func(t *testing.T, resultData map[string]any) {
			count := 0
			if c, ok := resultData["count"].(int); ok {
				count = c
			}

			matches := 0
			if m, ok := resultData["matches"].(int); ok {
				matches = m
			}

			assert.True(t, count > 0, "meshsync should publish Kubernetes objects after ReSync")
			assert.GreaterOrEqual(t, matches, minExpectedObjectsAfterResync, "ReSync should rediscover most of the existing cluster objects")

			if lastObject, has := resultData["last_object"]; has {
				if kr, ok := lastObject.(model.KubernetesResource); ok {
					t.Logf("Last object kind: %s", kr.Kind)
					t.Logf("Last object name: %s", kr.KubernetesResourceMeta.Name)
				}
			}

			t.Logf(
				"ReSync validation passed — %d objects published, %d matched previous discovery",
				count, matches,
			)
		},
	},
}
