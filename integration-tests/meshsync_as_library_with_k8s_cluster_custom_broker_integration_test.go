package tests

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/broker/channel"
	"github.com/meshery/meshkit/logger"
	libmeshsync "github.com/meshery/meshsync/pkg/lib/meshsync"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestMeshsyncLibraryWithK8sClusterCustomBrokerIntegration(t *testing.T) {
	if !runIntegrationTest {
		t.Skip("skipping integration test")
	}

	br := channel.NewChannelBrokerHandler()

	for i, tc := range meshsyncLibraryWithK8SClusterCustomBrokerTestCaseData {
		t.Run(
			tc.name,
			runWithMeshsyncLibraryAndk8sClusterCustomBrokerTestCase(
				br,
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
func runWithMeshsyncLibraryAndk8sClusterCustomBrokerTestCase(
	br broker.Handler,
	tcIndex int,
	tc meshsyncLibraryWithK8SClusterCustomBrokerTestCaseStruct,
) func(t *testing.T) {
	return func(t *testing.T) {
		for _, cleanupHook := range tc.cleanupHooks {
			defer cleanupHook()
		}

		for _, setupHook := range tc.setupHooks {
			setupHook()
		}

		loggerOptions, deferFunc := withMeshsyncLibraryAndk8sClusterCustomBrokerPrepareMeshsyncLoggerOptions(t, tcIndex)
		defer deferFunc()

		// Initialize Logger instance
		log, errLoggerNew := logger.New(
			fmt.Sprintf("TestWithMeshsyncLibraryAndK8sClusterIntegration-%02d", tcIndex),
			loggerOptions,
		)
		if errLoggerNew != nil {
			t.Fatal("must not end with error when creating logger", errLoggerNew)
		}

		errCh := make(chan error)
		out := make(chan *broker.Message)

		// Step 1: subscribe to the queue
		if err := br.SubscribeWithChannel(
			testMeshsyncTopic,
			// impotant to have a different queue group per test case
			// so that every test case receive message for each event
			fmt.Sprintf("meshsync-as-library-queue-group-%02d", tcIndex),
			out,
		); err != nil {
			t.Fatalf("error subscribing to topic: %v", err)
		}

		// Step 2: process messages
		resultData := make(map[string]any)
		if tc.brokerMessageHandler != nil {
			go tc.brokerMessageHandler(t, out, resultData)
		}

		// Step 3: run meshsync library
		go func(errCh0 chan<- error) {
			runOptions := make([]libmeshsync.OptionsSetter, 0, len(tc.meshsyncRunOptions))
			runOptions = append(runOptions, tc.meshsyncRunOptions...)
			runOptions = append(runOptions, libmeshsync.WithBrokerHandler(br))

			errCh0 <- libmeshsync.Run(
				log,
				runOptions...,
			)
		}(errCh)

		// intentionally big timeout to wait till the run execution ended
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

		// Step 4: do final assertion, if any
		if tc.finalHandler != nil {
			tc.finalHandler(t, resultData)
		}

		t.Logf("done %s", tc.name)
	}
}

func withMeshsyncLibraryAndk8sClusterCustomBrokerPrepareMeshsyncLoggerOptions(
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
		meshsyncOutputFileName := fmt.Sprintf("meshsync-as-library-with-k8s-cluster-custom-broker-test-case-%02d.meshsync-output.txt", tcIndex)
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
