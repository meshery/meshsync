package tests

import (
	"testing"
)

var k8sClusterMeshsyncAsLibraryTestCasesData []k8sClusterTestCaseStruct

func init() {
	for _, tcs := range [][]k8sClusterTestCaseStruct{
		k8sClusterTestCasesChannelModeData,
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
			runWithMeshsyncLibraryAndK8sClusterTestCase(
				i,
				tc,
			),
		)
	}
}

func runWithMeshsyncLibraryAndK8sClusterTestCase(
	tcIndex int,
	tc k8sClusterTestCaseStruct,
) func(t *testing.T) {
	return func(t *testing.T) {}
}
