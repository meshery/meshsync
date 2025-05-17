package tests

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/layer5io/meshkit/broker"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

type defaultClusterTestCaseStruct struct {
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

var defaultClusterTestCasesData []defaultClusterTestCaseStruct = []defaultClusterTestCaseStruct{
	{
		name:            "number of messages received from nats is greater than zero",
		meshsyncCMDArgs: []string{"--stopAfterSeconds", "8"},
		natsMessageHandler: func(
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
				assert.True(t, count > 0, "must receive messages from queue")
			}

		},
	},
	{
		name: "receive from nats only specified resources",
		meshsyncCMDArgs: []string{
			"--stopAfterSeconds",
			"8",
			"--outputResources",
			"pod,replicaset",
		},
		natsMessageHandler: func(
			t *testing.T,
			out chan *broker.Message,
			resultData map[string]any,
		) {
			resourcesCount := make(map[string]int)
			resultData["count"] = resourcesCount
			errCh := make(chan error)
			go func(errCh0 chan<- error) {
				for message := range out {
					k8sResource, err := unmarshalObject(message.Object)
					if err != nil {
						errCh0 <- fmt.Errorf(
							"error convering message.Object to model.KubernetesResource for %T",
							message.Object,
						)
						return
					}
					resourcesCount[strings.ToLower(k8sResource.Kind)]++
					resultData["count"] = resourcesCount
				}
			}(errCh)

			err := <-errCh
			if err != nil {
				t.Fatal(err)
			}
		},
		finalHandler: func(t *testing.T, resultData map[string]any) {
			count, ok := resultData["count"].(map[string]int)
			assert.True(t, ok, "must get count from result map")
			if ok {
				allowedKeys := map[string]bool{"pod": true, "replicaset": true}
				otherKeys := make([]string, 0)
				for k, v := range count {
					t.Logf("received %d messages from Kind %s", v, k)
					if !allowedKeys[k] {
						otherKeys = append(
							otherKeys,
							fmt.Sprintf("[%s = %v]", k, v),
						)
					}
				}
				assert.True(t, count["pod"] > 0, "must receive kind pod messages from queue")
				assert.True(t, count["replicaset"] > 0, "must receive kind replicaset messages from queue")
				if len(otherKeys) > 0 {
					t.Fatalf("received not allowed kind keys %s", strings.Join(otherKeys, ","))
				}
			}

		},
	},
	{
		name: "receive from nats only resources from specified namespace",
		meshsyncCMDArgs: []string{
			"--stopAfterSeconds",
			"8",
			"--outputNamespace",
			"agile-otter",
		},
		natsMessageHandler: func(
			t *testing.T,
			out chan *broker.Message,
			resultData map[string]any,
		) {
			resourcesPerNamespaceCount := make(map[string]int)
			resultData["count"] = resourcesPerNamespaceCount
			errCh := make(chan error)
			go func(errCh0 chan<- error) {
				for message := range out {
					k8sResource, err := unmarshalObject(message.Object)
					if err != nil {
						errCh0 <- fmt.Errorf(
							"error convering message.Object to model.KubernetesResource for %T",
							message.Object,
						)
						return
					}
					resourcesPerNamespaceCount[strings.ToLower(k8sResource.KubernetesResourceMeta.Namespace)]++
					resultData["count"] = resourcesPerNamespaceCount
				}
			}(errCh)

			err := <-errCh
			if err != nil {
				t.Fatal(err)
			}
		},
		finalHandler: func(t *testing.T, resultData map[string]any) {
			count, ok := resultData["count"].(map[string]int)
			assert.True(t, ok, "must get count from result map")
			if ok {
				allowedKeys := map[string]bool{"agile-otter": true}
				otherKeys := make([]string, 0)
				for k, v := range count {
					t.Logf("received %d messages for namespace %s", v, k)
					if !allowedKeys[k] {
						otherKeys = append(
							otherKeys,
							fmt.Sprintf("[%s = %v]", k, v),
						)
					}
				}
				assert.True(t, count["agile-otter"] > 0, "must receive messages from resources in agile-otter namespace")

				if len(otherKeys) > 0 {
					t.Fatalf("received not allowed namespace keys %s", strings.Join(otherKeys, ","))
				}
			}

		},
	},
	{
		name: "output mode file: must not receive message from queue",
		meshsyncCMDArgs: []string{
			"--stopAfterSeconds", "8",
			"--output", "file",
			"--outputFile", "meshery-cluster-snapshot-integration-test-03.yaml",
		},
		setupHooks: []func(){
			func() {
				// put this in setupHooks and not in cleanupHooks
				// as it is convinient to have files stay after test run
				// but need to clear them before the test run
				os.RemoveAll("meshery-cluster-snapshot-integration-test-03-extended.yaml")
				os.RemoveAll("meshery-cluster-snapshot-integration-test-03.yaml")
			},
		},
		natsMessageHandler: func(
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
		// unable to decode "integration-tests/meshery-cluster-snapshot-integration-test-04.yaml": json: cannot unmarshal array into Go struct field ObjectMeta.metadata.labels of type map[string]string
		name: "output mode file: must have yaml with kind pod and kind deployment only and from dedicated namespace",
		meshsyncCMDArgs: []string{
			"--stopAfterSeconds", "8",
			"--output", "file",
			"--outputResources", "pod,deployment",
			"--outputNamespace", "agile-otter",
			"--outputFile", "meshery-cluster-snapshot-integration-test-04.yaml",
		},
		setupHooks: []func(){
			func() {
				// put this in setupHooks and not in cleanupHooks
				// as it is convinient to have files stay after test run
				// but need to clear them before the test run
				os.RemoveAll("meshery-cluster-snapshot-integration-test-04-extended.yaml")
				os.RemoveAll("meshery-cluster-snapshot-integration-test-04.yaml")
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
			data, err := os.ReadFile("meshery-cluster-snapshot-integration-test-04.yaml")
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
	{
		name: "must not fail with a --broker-url param",
		meshsyncCMDArgs: []string{
			"--broker-url", "10.96.235.19:4222",
			"--stopAfterSeconds", "8",
		},
		natsMessageHandler: func(
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
				assert.True(t, count > 0, "must receive messages from queue")
			}

		},
	},
}
