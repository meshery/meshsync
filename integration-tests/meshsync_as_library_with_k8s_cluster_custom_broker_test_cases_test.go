package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	meshkitutils "github.com/meshery/meshkit/utils"
	mesherykube "github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/meshsync/internal/config"
	libmeshsync "github.com/meshery/meshsync/pkg/lib/meshsync"
	"github.com/meshery/meshsync/pkg/lib/tmp_meshkit/broker/channel"
	iutils "github.com/meshery/meshsync/pkg/utils"
	"github.com/stretchr/testify/assert"
)

var meshsyncLibraryWithK8SClusterCustomBrokerTestCaseData []meshsyncLibraryWithK8SClusterCustomBrokerTestCaseStruct = []meshsyncLibraryWithK8SClusterCustomBrokerTestCaseStruct{
	{
		name: "output mode channel: number of messages received from meshsync is greater than zero",
		meshsyncRunOptions: []libmeshsync.OptionsSetter{
			libmeshsync.WithOutputMode(config.OutputModeBroker),
			libmeshsync.WithBrokerHandler(channel.NewTMPChannelBrokerHandler()),
			libmeshsync.WithStopAfterDuration(8 * time.Second),
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
				t.Logf("received %d messages from meshsync", count)
				assert.True(t, count > 0, "must receive messages from meshsync")
			}

		},
	},
	// TODO
	// remove this as a separate test case,
	// add nil to []libmeshsync.OptionsSetter in the previous test case with a comment
	{
		name: "output mode channel: must not fail when has nil in options setter",
		meshsyncRunOptions: []libmeshsync.OptionsSetter{
			nil,
			libmeshsync.WithOutputMode(config.OutputModeBroker),
			libmeshsync.WithBrokerHandler(channel.NewTMPChannelBrokerHandler()),
			libmeshsync.WithStopAfterDuration(0 * time.Second),
		},
	},
	// TODO
	// this is not an output mode test
	// we do not need to run libmeshsync.Run, as we only test call to GetClusterID
	// we still need k8s cluster in place;
	// maybe think about to move in a separate flow.
	{
		name: "output mode channel: can get clusterID from utils function",
		meshsyncRunOptions: []libmeshsync.OptionsSetter{
			libmeshsync.WithOutputMode(config.OutputModeBroker),
			libmeshsync.WithBrokerHandler(channel.NewTMPChannelBrokerHandler()),
			libmeshsync.WithStopAfterDuration(0 * time.Second),
		},
		finalHandler: func(t *testing.T, resultData map[string]any) {
			kubeClient, err := mesherykube.New(nil)
			assert.NoError(t, err)
			if err == nil {
				clusterID := iutils.GetClusterID(kubeClient.KubeClient)
				t.Logf("clusterId = %s", clusterID)
				assert.NotEmpty(t, clusterID)
			}
		},
	},
	{
		name: "output mode channel: can access cluster when receive kube config from options",
		meshsyncRunOptions: []libmeshsync.OptionsSetter{
			libmeshsync.WithOutputMode(config.OutputModeBroker),
			libmeshsync.WithBrokerHandler(channel.NewTMPChannelBrokerHandler()),
			// read the kube config and provide its content through libmeshsync.WithKubeConfig
			func() libmeshsync.OptionsSetter {
				kubeConfigFilePath := os.Getenv("KUBECONFIG")
				if kubeConfigFilePath == "" {
					kubeConfigFilePath = filepath.Join(
						meshkitutils.GetHome(),
						".kube",
						"config",
					)
					fmt.Println(kubeConfigFilePath)
				}
				data, _ := os.ReadFile(kubeConfigFilePath)
				if data == nil {
					// because if data is nil, meshsync will read from default kube config
					// and we would like to test from custom provided
					data = fmt.Appendf(nil, "could not read from kube config %s", kubeConfigFilePath)
				}
				return libmeshsync.WithKubeConfig(data)
			}(),
			libmeshsync.WithStopAfterDuration(8 * time.Second),
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
				t.Logf("received %d messages from meshsync", count)
				assert.True(t, count > 0, "must receive messages from meshsync")
			}

		},
	},
	{
		name: "output mode channel: could not access cluster with invalid kubeconfig",
		meshsyncRunOptions: []libmeshsync.OptionsSetter{
			libmeshsync.WithOutputMode(config.OutputModeBroker),
			libmeshsync.WithBrokerHandler(channel.NewTMPChannelBrokerHandler()),
			libmeshsync.WithKubeConfig([]byte(`fake kube config`)),
			libmeshsync.WithStopAfterDuration(0 * time.Second),
		},
		expectError:          true,
		expectedErrorMessage: "cannot unmarshal string into Go value of type struct",
	},
}
