package tests

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/broker/nats"
)

var meshsyncAsBinaryWithK8SClusterTestCasesData []meshsyncBinaryWithK8SClusterTestsCasesStruct

func init() {
	for _, tcs := range [][]meshsyncBinaryWithK8SClusterTestsCasesStruct{
		meshsyncBinaryWithK8SClusterBrokerModeTestsCasesData,
		meshSyncBinaryWithK8SClusterFileModeTestCasesData,
	} {
		meshsyncAsBinaryWithK8SClusterTestCasesData = append(
			meshsyncAsBinaryWithK8SClusterTestCasesData,
			tcs...,
		)
	}
}

func TestMeshsyncBinaryWithK8sClusterIntegration(t *testing.T) {
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

	for i, tc := range meshsyncAsBinaryWithK8SClusterTestCasesData {
		t.Run(
			tc.name,
			runMeshsyncBinaryWithK8sClusterTestCase(
				br,
				i,
				tc,
			),
		)
	}
}

func runMeshsyncBinaryWithK8sClusterTestCase(
	br broker.Handler,
	tcIndex int,
	tc meshsyncBinaryWithK8SClusterTestsCasesStruct,
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
			// impotant to have a different queue group per test case
			// so that every test case receive message for each event
			fmt.Sprintf("meshsync-as-binary-queue-group-%02d", tcIndex),
			out,
		); err != nil {
			t.Fatalf("error subscribing to topic: %v", err)
		}

		// Step 2: process messages
		resultData := make(map[string]any)
		if tc.brokerMessageHandler != nil {
			go tc.brokerMessageHandler(t, out, resultData)
		}

		os.Setenv("BROKER_URL", testMeshsyncNatsURL)

		// Step 3: run the meshsync command
		cmd, deferFunc := withMeshsyncBinaryPrepareMeshsyncCMD(t, tcIndex, tc)
		defer deferFunc()

		if err := cmd.Start(); err != nil {
			t.Fatalf("error starting binary: %v", err)
		}

		// intentionally big timeout to wait till the cmd execution ended
		timeout := time.Duration(time.Hour * 24)
		if tc.waitMeshsyncTimeout > 0 {
			timeout = tc.waitMeshsyncTimeout
		}

		waitForMeshsync(t, cmd, timeout)

		// Step 4: do final assertion, if any
		if tc.finalHandler != nil {
			tc.finalHandler(t, resultData)
		}

		t.Logf("done %s", tc.name)
	}
}

// introduced these below function to decrease cyclomatic complexity
func withMeshsyncBinaryPrepareMeshsyncCMD(
	t *testing.T,
	tcIndex int,
	tc meshsyncBinaryWithK8SClusterTestsCasesStruct,
) (*exec.Cmd, func()) {
	cmd := exec.Command(meshsyncBinaryPath, tc.meshsyncCMDArgs...)
	deferFunc := func() {}
	// there is quite rich output from meshsync
	// save to file instead of stdout
	if saveMeshsyncOutput {
		meshsyncOutputFileName := fmt.Sprintf("meshsync-as-binary-with-k8s-cluster-test-case-%02d.meshsync-output.txt", tcIndex)
		meshsyncOutputFile, err := os.Create(meshsyncOutputFileName)
		if err != nil {
			t.Logf("Could not create meshsync output file %s", meshsyncOutputFileName)
			// if not possible to create output file, print to the stdout
			cmd.Stdout = os.Stdout
		} else {
			deferFunc = func() {
				meshsyncOutputFile.Close()
			}
			cmd.Stdout = meshsyncOutputFile
		}
	} else {
		cmd.Stdout = nil
	}
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd, deferFunc
}

func waitForMeshsync(
	t *testing.T,
	cmd *exec.Cmd,
	timeout time.Duration,
) {
	errCh := make(chan error)

	go func() {
		errCh <- cmd.Wait()
	}()

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
}
