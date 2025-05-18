package tests

import (
	"os"
	"testing"
	"time"

	"github.com/layer5io/meshkit/broker"
)

/**
 * to run locally this tests require:
 * - docker
 * - kind
 * - kubectl
 * --
 * use Makefile to run
 * --
 * this test runs all test cases on the same k8s cluster, but with different input params for meshsync;
 * if you need a specific cluster setup you (probably) need to write a separate test,
 * or fit in the current cluster set up without failing existing tests;
 * --
 * test flow of every test case is as follow:
 * - subscribe to nats (each test case has a separate queue group, so it receives every message);
 * - run meshsync binary;
 * - receive messages from nats and perform assertions;
 */

var runIntegrationTest bool
var meshsyncBinaryPath string
var saveMeshsyncOutput bool // if true, saves outputof meshsync binary to file
var testMeshsyncTopic = "meshery.meshsync.core"
var testMeshsyncNatsURL = "localhost:4222"

func init() {
	runIntegrationTest = os.Getenv("RUN_INTEGRATION_TESTS") == "true"
	meshsyncBinaryPath = os.Getenv("MESHSYNC_BINARY_PATH")
	saveMeshsyncOutput = os.Getenv("SAVE_MESHSYNC_OUTPUT") == "true"
}

type k8sClusterTestCaseStruct struct {
	setupHooks          []func()
	cleanupHooks        []func()
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
