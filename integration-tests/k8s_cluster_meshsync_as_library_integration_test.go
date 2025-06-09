package tests

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/meshery/meshkit/logger"
	"github.com/meshery/meshsync/internal/output"
	libmeshsync "github.com/meshery/meshsync/pkg/lib/meshsync"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

var k8sClusterMeshsyncAsLibraryTestCasesData []k8sClusterMeshsyncLibraryTestCaseStruct

func init() {
	for _, tcs := range [][]k8sClusterMeshsyncLibraryTestCaseStruct{
		k8sClusterMeshsyncLibraryTestCasesChannelModeData,
	} {
		k8sClusterMeshsyncAsLibraryTestCasesData = append(
			k8sClusterMeshsyncAsLibraryTestCasesData,
			tcs...,
		)
	}
}

func TestWithMeshsyncLibraryAndK8sClusterIntegration(t *testing.T) {
	if !runIntegrationTest {
		t.Skip("skipping integration test")
	}

	for i, tc := range k8sClusterMeshsyncAsLibraryTestCasesData {
		t.Run(
			tc.name,
			runWithMeshsyncLibraryAndk8sClusterTestCase(
				i,
				tc,
			),
		)
	}
}

// TODO fix cyclop error
// integration-tests/k8s_cluster_meshsync_as_library_integration_test.go:47:1: calculated cyclomatic complexity for function runWithMeshsyncLibraryAndk8sClusterTestCase is 15, max is 10 (cyclop)
//
//nolint:cyclop
func runWithMeshsyncLibraryAndk8sClusterTestCase(
	tcIndex int,
	tc k8sClusterMeshsyncLibraryTestCaseStruct,
) func(t *testing.T) {
	return func(t *testing.T) {
		for _, cleanupHook := range tc.cleanupHooks {
			defer cleanupHook()
		}

		for _, setupHook := range tc.setupHooks {
			setupHook()
		}

		loggerOptions, deferFunc := withMeshsyncLibraryPrepareMeshsyncLoggerOptions(t, tcIndex)
		defer deferFunc()

		// Initialize Logger instance
		log, errLoggerNew := logger.New(
			fmt.Sprintf("TestWithMeshsyncLibraryAndK8sClusterIntegration-%02d", tcIndex),
			loggerOptions,
		)
		if errLoggerNew != nil {
			t.Fatal("must not end with error when creating logger", errLoggerNew)
		}

		// prepare transport and error channels, result map
		transportCh := make(chan *output.ChannelItem, 1024)
		errCh := make(chan error)
		resultData := make(map[string]any)

		// Step 1: run meshsync channel message handler
		if tc.channelMessageHandler != nil {
			go tc.channelMessageHandler(t, transportCh, resultData)
		}

		// Step 2: run meshsync library
		go func(errCh0 chan<- error) {
			runOptions := make([]libmeshsync.OptionsSetter, 0, len(tc.meshsyncRunOptions))
			runOptions = append(runOptions, tc.meshsyncRunOptions...)
			runOptions = append(runOptions, libmeshsync.WithTransportChannel(transportCh))

			errCh0 <- libmeshsync.Run(
				log,
				runOptions...,
			)
		}(errCh)

		// intentionally big timeout to wait till the cmd execution ended
		timeout := time.Duration(time.Hour * 24)
		if tc.waitMeshsyncTimeout > 0 {
			timeout = tc.waitMeshsyncTimeout
		}

		select {
		case err := <-errCh:
			if err != nil {
				if !tc.expectError {
					t.Fatal("must not end with error", err)
				}
				assert.ErrorContains(t, err, tc.expectedErrorMessage, "must end with expected error")
			} else if tc.expectError {
				if tc.expectedErrorMessage != "" {
					t.Fatalf("must end with expected error message %s", tc.expectedErrorMessage)
				}
				t.Fatalf("must end with error")
			}
		case <-time.After(timeout):
			self, err := os.FindProcess(os.Getpid())
			if err != nil {
				t.Fatalf("could not find self process: %v", err)
			}
			if err := self.Signal(syscall.SIGTERM); err != nil {
				t.Fatalf("error terminating meshsync library: %v", err)
			}
			t.Logf("processing after timeout %d", timeout)
		}

		// Step 3: do final assertion, if any
		if tc.finalHandler != nil {
			tc.finalHandler(t, resultData)
		}

		t.Logf("done %s", tc.name)
	}
}

func withMeshsyncLibraryPrepareMeshsyncLoggerOptions(
	t *testing.T,
	tcIndex int,
) (logger.Options, func()) {
	options := logger.Options{
		Format:   logger.SyslogLogFormat,
		LogLevel: int(logrus.InfoLevel),
	}
	deferFunc := func() {}
	// there is quite rich output from meshsync
	// save to file instead of stdout
	if saveMeshsyncOutput {
		meshsyncOutputFileName := fmt.Sprintf("k8s-cluster-meshsync-as-library-test-case-%02d.meshsync-output.txt", tcIndex)
		meshsyncOutputFile, err := os.Create(meshsyncOutputFileName)
		if err != nil {
			t.Logf("Could not create meshsync output file %s", meshsyncOutputFileName)
			// if not possible to create output file, leave default output for logger
		} else {
			deferFunc = func() {
				meshsyncOutputFile.Close()
			}
			options.Output = meshsyncOutputFile
		}
	} else {
		options.Output = io.Discard
	}

	return options, deferFunc
}
