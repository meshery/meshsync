package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/layer5io/meshkit/broker"
	"github.com/stretchr/testify/assert"
)

type defaultClusterTestCaseStruct struct {
	name                string
	meshsyncCMDArgs     []string      // args to pass to meshsync binary
	waitMeshsyncTimeout time.Duration // if <= 0: waits till meshsync ends execution, otherwise moves  further after specified duration
	// the reason for resultData map is that natsMessageHandler is processing chan indefinitely
	// and there is no graceful exit from function;
	natsMessageHandler func(
		t *testing.T,
		out chan *broker.Message,
		resultData map[string]any,
	)
	finalHandler func(t *testing.T, resultData map[string]any)
}

var defaultClusterTestCasesData []defaultClusterTestCaseStruct = []defaultClusterTestCaseStruct{
	{
		name:            "number of messages received from nats is greater than zero",
		meshsyncCMDArgs: []string{"--stopAfterSeconds", "8"},
		natsMessageHandler: func(
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
				// TODO some more meaningful check
				assert.True(t, count > 0, "must receive messages from queue")
			}

		},
	},
	{
		name: "receive from nats only specified resources",
		meshsyncCMDArgs: []string{
			"--stopAfterSeconds",
			"8",
			"--outputResources",
			"pod,replicaset",
		},
		natsMessageHandler: func(
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
				// TODO some more meaningful check
				assert.True(t, count["pod"] > 0, "must receive kind pod messages from queue")
				assert.True(t, count["replicaset"] > 0, "must receive kind replicaset messages from queue")
				if len(otherKeys) > 0 {
					t.Fatalf("received not allowed keys %s", strings.Join(otherKeys, ","))
				}
			}

		},
	},
}
