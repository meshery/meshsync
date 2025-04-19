package main

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/broker/nats"
	"github.com/stretchr/testify/assert"
)

var runIntegrationTest bool
var meshsyncBinaryPath string
var testMeshsyncTopic = "meshery.meshsync.core"
var testMeshsyncNatsURL = "localhost:4222"

func init() {
	runIntegrationTest = os.Getenv("RUN_INTEGRATION_TESTS") == "true"
	meshsyncBinaryPath = os.Getenv("MESHSYNC_BINARY_PATH")
}

/**
 * this test requires k8s cluster (with installed CRDs: meshsync) and nats streaming
 * --
 * use docker compose to start nats
 * ---
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

	out := make(chan *broker.Message)
	err = br.SubscribeWithChannel(testMeshsyncTopic, "", out)
	if err != nil {
		t.Fatalf("error subscribing to topic: %v", err)
	}

	go func() {
		for range out {
			count++
		}
	}()

	os.Setenv("BROKER_URL", testMeshsyncNatsURL)

	// Create the command
	args := []string{"--stopAfterSeconds", "8"}
	cmd := exec.Command(meshsyncBinaryPath, args...)

	// Set the output to be the same as the current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Run it
	err = cmd.Run()
	if err != nil {
		t.Fatalf("error running binary: %v", err)
	}

	// TODO some more meaningful check
	assert.True(t, count > 0)

	t.Logf("received %d messages from broker", count)
	t.Log("done")
}
