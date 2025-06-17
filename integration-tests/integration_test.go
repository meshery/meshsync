package tests

import (
	"os"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	libmeshsync "github.com/meshery/meshsync/pkg/lib/meshsync"
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

type meshsyncBinaryWithK8SClusterTestsCasesStruct struct {
	setupHooks          []func()
	cleanupHooks        []func()
	name                string
	meshsyncCMDArgs     []string      // args to pass to meshsync binary
	waitMeshsyncTimeout time.Duration // if <= 0: waits till meshsync ends execution, otherwise moves  further after specified duration
	// the reason for resultData map is that brokerMessageHandler is processing chan indefinitely
	// and there is no graceful exit from function;
	brokerMessageHandler func(
		t *testing.T,
		out chan *broker.Message,
		resultData map[string]any,
	)
	finalHandler func(t *testing.T, resultData map[string]any)
}

type meshsyncLibraryWithK8SClusterCustomBrokerTestCaseStruct struct {
	setupHooks           []func()
	cleanupHooks         []func()
	name                 string
	waitMeshsyncTimeout  time.Duration // if <= 0: waits till meshsync ends execution, otherwise moves  further after specified duration
	meshsyncRunOptions   []libmeshsync.OptionsSetter
	brokerMessageHandler func(
		t *testing.T,
		out chan *broker.Message,
		resultData map[string]any,
	) // result map is to propagate data between channelMessageHandler and finalHandler
	finalHandler         func(t *testing.T, resultData map[string]any)
	expectError          bool   // if this is true, result run expected to end with error
	expectedErrorMessage string // if this is not "", check that return error contains message, makes sense only if expectError = true
}
