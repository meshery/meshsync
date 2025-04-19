package tests

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/broker/nats"
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
 * this test requires k8s cluster (with installed CRDs: meshsync) and nats streaming;
 * --
 * use docker compose to start nats;
 * ---
 * this test runs all test cases on the same k8s cluster, but with different input params for meshsync;
 * if you need a specific cluster setup you (probably) need to write a separate test,
 * or fit in the current cluster set up without failing existing tests;
 * --
 * test flow of every test case is as follow:
 * - subscribe to nats (each test case has a separate queue group, so it receives every message);
 * - run meshsync binary;
 * - receive messages from nats and perform assertions;
 */
func TestWithNatsDefaultK8SClusterIntegration(t *testing.T) {
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

	for i, tc := range defaultClusterTestCasesData {
		t.Run(tc.name, func(t *testing.T) {
			out := make(chan *broker.Message)
			// Step 1: subscribe to the queue
			err = br.SubscribeWithChannel(
				testMeshsyncTopic,
				fmt.Sprintf("default-cluster-queue-group-%d", i),
				out,
			)
			if err != nil {
				t.Fatalf("error subscribing to topic: %v", err)
			}

			// Step 2: process messages
			resultData := make(map[string]any, 1)
			go tc.natsMessageHandler(t, out, resultData)

			os.Setenv("BROKER_URL", testMeshsyncNatsURL)

			// Step 3: run the meshsync command
			cmd := exec.Command(meshsyncBinaryPath, tc.meshsyncCMDArgs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			if err := cmd.Start(); err != nil {
				t.Fatalf("error starting binary: %v", err)
			}
			errCh := make(chan error)
			go func(cmd0 *exec.Cmd, errCh0 chan<- error) {
				errCh0 <- cmd0.Wait()
			}(cmd, errCh)

			// intentionally big timeout to wait till the cmd execution ended
			timeout := time.Duration(time.Hour * 24)
			if tc.waitMeshsyncTimeout > 0 {
				timeout = tc.waitMeshsyncTimeout
			}

			select {
			case err := <-errCh:
				if err != nil {
					t.Fatalf("error running binary: %v", err)
				}
			case <-time.After(timeout):
				if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatalf("error terminating meshsync command: %v", err)
				}
				t.Logf("processing after timeout %d", timeout)
			}

			// Step 4: do final assertion, if any
			tc.finalHandler(t, resultData)

			t.Logf("done %s", tc.name)
		})
	}
}
