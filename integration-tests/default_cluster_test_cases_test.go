package tests

import (
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
		name:                "number of messages received from nats is greater than zero",
		waitMeshsyncTimeout: time.Duration(8 * time.Second),
		natsMessageHandler: func(
			t *testing.T,
			out chan *broker.Message,
			resultData map[string]any,
		) {
			count := 0
			resultData["count"] = 0
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
}
