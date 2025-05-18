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

var k8sClusterMeshsyncAsBinaryTestCasesData []k8sClusterMeshsyncBinaryTestCaseStruct

func init() {
	for _, tcs := range [][]k8sClusterMeshsyncBinaryTestCaseStruct{
		k8sClusterMeshsyncBinaryTestCasesNatsModeData,
		k8sClusterMeshsyncBinaryTestCasesFileModeData,
	} {
		k8sClusterMeshsyncAsBinaryTestCasesData = append(
			k8sClusterMeshsyncAsBinaryTestCasesData,
			tcs...,
		)
	}
}

func TestWithMeshsyncBinaryAndK8sClusterIntegration(t *testing.T) {
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

	for i, tc := range k8sClusterMeshsyncAsBinaryTestCasesData {
		t.Run(
			tc.name,
			runWithMeshsyncBinaryAndk8sClusterMeshsyncBinaryTestCase(
				br,
				i,
				tc,
			),
		)
	}
}

// need this as separate function to bring down cyclomatic complexity
// this one itself is also already too complicated :)
//
// TODO fix cyclop error
// integration-tests/k8s_cluster_integration_test.go:74:1: calculated cyclomatic complexity for function runWithMeshsyncBinaryAndk8sClusterMeshsyncBinaryTestCase is 11, max is 10 (cyclop)
//
//nolint:cyclop
func runWithMeshsyncBinaryAndk8sClusterMeshsyncBinaryTestCase(
	br broker.Handler,
	tcIndex int,
	tc k8sClusterMeshsyncBinaryTestCaseStruct,
) func(t *testing.T) {
	return func(t *testing.T) {
		for _, cleanupHook := range tc.cleanupHooks {
			defer cleanupHook()
		}

		for _, setupHook := range tc.setupHooks {
			setupHook()
		}

		out := make(chan *broker.Message)
		// Step 1: subscribe to the queue
		if err := br.SubscribeWithChannel(
			testMeshsyncTopic,
			fmt.Sprintf("k8s-cluster-meshsync-as-binary-queue-group-%d", tcIndex),
			out,
		); err != nil {
			t.Fatalf("error subscribing to topic: %v", err)
		}

		// Step 2: process messages
		resultData := make(map[string]any, 1)
		if tc.natsMessageHandler != nil {
			go tc.natsMessageHandler(t, out, resultData)
		}

		os.Setenv("BROKER_URL", testMeshsyncNatsURL)

		// Step 3: run the meshsync command
		cmd, deferFunc := prepareMeshsyncCMD(t, tcIndex, tc)
		defer deferFunc()

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
	}
}

// introduced this function to decrease cyclomatic complexity
func prepareMeshsyncCMD(
	t *testing.T,
	tcIndex int,
	tc k8sClusterMeshsyncBinaryTestCaseStruct,
) (*exec.Cmd, func()) {
	cmd := exec.Command(meshsyncBinaryPath, tc.meshsyncCMDArgs...)
	deferFunc := func() {}
	// there is quite rich output from meshsync
	// save to file instead of stdout
	if saveMeshsyncOutput {
		meshsyncOutputFileName := fmt.Sprintf("k8s-cluster-meshsync-as-binary-test-case-%02d.meshsync-output.txt", tcIndex)
		meshsyncOutputFile, err := os.Create(meshsyncOutputFileName)
		if err != nil {
			t.Logf("Could not create meshsync output file %s", meshsyncOutputFileName)
			// if not possible to create output file, print to the stdout
			cmd.Stdout = os.Stdout
		}
		deferFunc = func() {
			meshsyncOutputFile.Close()
		}
		cmd.Stdout = meshsyncOutputFile
	} else {
		cmd.Stdout = nil
	}
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd, deferFunc
}
