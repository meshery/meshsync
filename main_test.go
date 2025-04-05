package main

import (
	"os"
	"testing"
	"time"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/broker/nats"
	"github.com/stretchr/testify/assert"
)

var runIntegrationTest bool
var testMeshsyncTopic = "meshery.meshsync.core"
var testMeshsyncNatsURL = "localhost:4222"

func init() {
	runIntegrationTest = os.Getenv("RUN_INTEGRATION_TESTS") == "true"
}

/**
 * this test requires k8s cluster and nats streaming
 * could be a good idea to put a test into ci workflow:
 * - start a kind cluster and nats container
 * - check that the messages are received in nats
 * --
 * use docker compose to start nats
 * ---
 * TODO:
 * - add starting kind cluster to docker compose
 */
func TestWithNatsIntegration(t *testing.T) {
	if !runIntegrationTest {
		t.Skip("skipping integration test")
	}

	br, err := nats.New(nats.Options{
		URLS:           []string{testMeshsyncNatsURL},
		ConnectionName: "meshsync",
		Username:       "",
		Password:       "",
		ReconnectWait:  2 * time.Second,
		MaxReconnect:   60,
	})
	if err != nil {
		t.Fatal("error connecting to nats", err)
	}
	count := 0

	go func() {
		out := make(chan *broker.Message)
		br.SubscribeWithChannel(testMeshsyncTopic, "", out)

		for range out {
			count++
		}
	}()

	os.Setenv("BROKER_URL", testMeshsyncNatsURL)
	go main()

	<-time.After(time.Second * 8)
	// TODO some more meaningful check
	assert.True(t, count > 0)

	t.Logf("received %d messages from broker", count)
	t.Log("done")
}
