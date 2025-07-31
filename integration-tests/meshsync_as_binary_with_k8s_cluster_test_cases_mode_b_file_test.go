package tests

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/meshery/meshkit/broker"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

var meshSyncBinaryWithK8SClusterFileModeTestCasesData []meshsyncBinaryWithK8SClusterTestsCasesStruct = []meshsyncBinaryWithK8SClusterTestsCasesStruct{
	{
		name: "output mode file: must not receive message from queue",
		meshsyncCMDArgs: []string{
			"--stopAfter", "8s",
			"--output", "file",
			"--outputFile", "meshery-cluster-snapshot-integration-test-file-mode-00.yaml",
		},
		setupHooks: []func(){
			func() {
				// put this in setupHooks and not in cleanupHooks
				// as it is convinient to have files stay after test run
				// but need to clear them before the test run
				os.RemoveAll("meshery-cluster-snapshot-integration-test-file-mode-00-extended.yaml")
				os.RemoveAll("meshery-cluster-snapshot-integration-test-file-mode-00.yaml")
			},
		},
		brokerMessageHandler: func(
			t *testing.T,
			out chan *broker.Message,
			resultData map[string]any,
		) {
			count := 0
			resultData["count"] = count
			go func() {
				for range out {
					count++
					resultData["count"] = count
				}
			}()
		},
		finalHandler: func(t *testing.T, resultData map[string]any) {
			count, ok := resultData["count"].(int)
			assert.True(t, ok, "must get count from result map")
			if ok {
				t.Logf("received %d messages from broker", count)
				assert.Equal(t, 0, count, "must not receive messages from queue")
			}

		},
	},
	{
		// TODO check that yaml must be a valid k8s manifest
		// f.e. with kubectl apply --dry-run=client
		// now the yaml has an issues, when dry-run receive an error:
		// unable to decode "integration-tests/meshery-cluster-snapshot-integration-test-file-mode-01.yaml": json: cannot unmarshal array into Go struct field ObjectMeta.metadata.labels of type map[string]string
		name: "output mode file: must have yaml with kind pod and kind deployment only and from dedicated namespace",
		meshsyncCMDArgs: []string{
			"--stopAfter", "8s",
			"--output", "file",
			"--outputResources", "pod,deployment",
			"--outputNamespaces", "agile-otter",
			"--outputFile", "meshery-cluster-snapshot-integration-test-file-mode-01.yaml",
		},
		setupHooks: []func(){
			func() {
				// put this in setupHooks and not in cleanupHooks
				// as it is convinient to have files stay after test run
				// but need to clear them before the test run
				os.RemoveAll("meshery-cluster-snapshot-integration-test-file-mode-01-extended.yaml")
				os.RemoveAll("meshery-cluster-snapshot-integration-test-file-mode-01.yaml")
			},
		},
		finalHandler: func(t *testing.T, resultData map[string]any) {
			expectedKinds := []string{"pod", "deployment"}
			expectedNamespace := "agile-otter"

			type K8sMetadata struct {
				Kind     string `yaml:"kind"`
				Metadata struct {
					Namespace string `yaml:"namespace"`
				} `yaml:"metadata"`
			}

			// Load file content
			data, err := os.ReadFile("meshery-cluster-snapshot-integration-test-file-mode-01.yaml")
			if err != nil {
				panic(err)
			}

			// Split on '---' to handle multiple resources
			docs := strings.Split(string(data), "---")
			var objects []K8sMetadata

			for _, doc := range docs {
				doc = strings.TrimSpace(doc)
				if doc == "" {
					continue
				}

				var obj K8sMetadata
				if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
					fmt.Printf("Error parsing document: %v\n", err)
					continue
				}
				objects = append(objects, obj)

				kindCount := make(map[string]int)
				namespaceCount := make(map[string]int)
				for _, obj := range objects {
					kindCount[strings.ToLower(obj.Kind)]++
					namespaceCount[obj.Metadata.Namespace]++
				}

				for kind, count := range kindCount {
					t.Logf("read %d objects of Kind %s", count, kind)
					assert.Contains(t, expectedKinds, kind, "kind must be one from the expected list")
				}

				for namespace, count := range namespaceCount {
					t.Logf("read %d objects with namespace %s", count, namespace)
					assert.Equal(t, expectedNamespace, namespace, "namespace must match expected namespace")
				}
			}
		},
	},
}
