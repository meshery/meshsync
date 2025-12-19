package tests

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/pkg/model"
	"github.com/stretchr/testify/assert"
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
		meshsyncCMDArgs: []string{"--stopAfter", "25s"},

		brokerMessageHandler: func(t *testing.T, br broker.Handler, out chan *broker.Message, resultData map[string]any) {
			t.Helper()

			beforeResync := make(map[string]bool)
			afterResync := make(map[string]bool)

			const (
				discoveryDebounce      = 2 * time.Second
				resyncSuccessThreshold = 5
			)

			var debounceTimer *time.Timer
			discoveryComplete := make(chan struct{})
			discoveryOnce := sync.Once{}

			errCh := make(chan error, 1)

			resetDebounce := func() {
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(discoveryDebounce, func() {
					discoveryOnce.Do(func() {
						close(discoveryComplete)
					})
				})
			}
			// phase 1: discovery
			go func(errCh0 chan<- error) {
				for msg := range out {
					if msg == nil || msg.Object == nil {
						continue
					}

					if msg.EventType != broker.Add &&
						msg.EventType != broker.Update &&
						msg.EventType != broker.Delete {
						continue
					}

					kr, err := unmarshalObject(msg.Object)
					if err != nil {
						errCh0 <- fmt.Errorf("discovery unmarshal failed: %w", err)
						return
					}

					if kr.Kind == "" {
						continue
					}

					key := kr.Kind + ":" + kr.KubernetesResourceMeta.Name
					beforeResync[key] = true
					resetDebounce()

					select {
					case <-discoveryComplete:
						return
					default:
					}
				}
			}(errCh)

			select {
			case <-discoveryComplete:
				t.Logf(
					"Initial discovery quiesced (%d objects)",
					len(beforeResync),
				)
			case err := <-errCh:
				t.Fatal(err)
			}

			// phase 2: trigger resync
			err := br.Publish(
				"meshery.meshsync.request",
				&broker.Message{
					Request: &broker.RequestObject{
						Entity: broker.RequestEntity("resync-discovery"),
					},
				},
			)
			if err != nil {
				t.Fatalf("failed to publish ReSync request: %v", err)
			}

			resultData["resync_requested"] = true
			t.Log("ReSync request published")

			// phase 3: after resync
			for msg := range out {
				if msg == nil || msg.Object == nil {
					continue
				}

				if msg.EventType != broker.Add &&
					msg.EventType != broker.Update {
					continue
				}

				kr, err := unmarshalObject(msg.Object)
				if err != nil {
					t.Fatalf("post-resync unmarshal failed: %v", err)
				}

				if kr.Kind == "" {
					continue
				}

				key := kr.Kind + ":" + kr.KubernetesResourceMeta.Name
				afterResync[key] = true
				resultData["last_object"] = kr

				if len(afterResync) >= resyncSuccessThreshold {
					break
				}
			}

			// results
			matchCount := 0
			for k := range beforeResync {
				if afterResync[k] {
					matchCount++
				}
			}

			resultData["count"] = len(afterResync)
			resultData["matches"] = matchCount
			resultData["received"] = len(afterResync) > 0

			t.Logf(
				"ReSync completed — After: %d objects | Matched: %d",
				len(afterResync),
				matchCount,
			)
		},
		finalHandler: func(t *testing.T, resultData map[string]any) {
			const minExpectedObjectsAfterResync = 3
			count := 0
			if c, ok := resultData["count"].(int); ok {
				count = c
			}

			matches := 0
			if m, ok := resultData["matches"].(int); ok {
				matches = m
			}

			assert.True(t, count > 0, "meshsync should publish Kubernetes objects after ReSync")

			assert.GreaterOrEqual(
				t,
				matches,
				minExpectedObjectsAfterResync,
				"ReSync should rediscover most existing cluster objects",
			)

			if lastObject, has := resultData["last_object"]; has {
				if kr, ok := lastObject.(model.KubernetesResource); ok {
					t.Logf("Last object kind: %s", kr.Kind)
					t.Logf("Last object name: %s", kr.KubernetesResourceMeta.Name)
				}
			}

			t.Logf(
				"ReSync validation passed — %d objects republished, %d matched",
				count,
				matches,
			)
		},
	},
}
