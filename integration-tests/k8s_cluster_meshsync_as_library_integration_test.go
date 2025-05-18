package tests

import (
	"testing"
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

	// // Initialize Logger instance
	// log, errLoggerNew := logger.New("TestWithMeshsyncLibraryAndK8sClusterIntegration", logger.Options{
	// 	Format:   logger.SyslogLogFormat,
	// 	LogLevel: int(logrus.InfoLevel),
	// })
	// if errLoggerNew != nil {
	// 	t.Fatal("must not end with error when creating logger", errLoggerNew)
	// }

	// if err := libmeshsync.Run(
	// 	log,
	// 	libmeshsync.WithStopAfterDuration(config.StopAfterDuration),
	// ); err != nil {
	// 	log.Error(err)
	// 	os.Exit(1)
	// }

	// for i, tc := range k8sClusterMeshsyncAsLibraryTestCasesData {
	// 	t.Run(
	// 		tc.name,
	// 		runWithMeshsyncLibraryAndk8sClusterMeshsyncBinaryTestCase(
	// 			i,
	// 			tc,
	// 		),
	// 	)
	// }
}

func runWithMeshsyncLibraryAndk8sClusterMeshsyncBinaryTestCase(
	tcIndex int,
	tc k8sClusterMeshsyncBinaryTestCaseStruct,
) func(t *testing.T) {
	return func(t *testing.T) {}
}
