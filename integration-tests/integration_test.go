package tests

import (
	"os"
	"testing"
	"time"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/output"
	libmeshsync "github.com/layer5io/meshsync/pkg/lib/meshsync"
)

/**
 * to run locally this tests require:
 * - docker
 * - kind
 * - kubectl
 * --
 * use Makefile to run
 * --
 * this tests runs all test cases on the same k8s cluster, but with different input params for meshsync;
 * if you need a specific cluster setup you (probably) need to write a separate tests,
 * or fit in the current cluster set up without failing existing tests;
 * --
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

type k8sClusterMeshsyncBinaryTestCaseStruct struct {
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

type k8sClusterMeshsyncLibraryTestCaseStruct struct {
	setupHooks            []func()
	cleanupHooks          []func()
	name                  string
	waitMeshsyncTimeout   time.Duration // if <= 0: waits till meshsync ends execution, otherwise moves  further after specified duration
	meshsyncRunOptions    []libmeshsync.OptionsSetter
	channelMessageHandler func(
		t *testing.T,
		out chan *output.ChannelItem,
		resultData map[string]any,
	) // result map is to propagate data between channelMessageHandler and finalHandler
	finalHandler  func(t *testing.T, resultData map[string]any)
	expectedError error // if this is not nil, result run expected to end with error
}
