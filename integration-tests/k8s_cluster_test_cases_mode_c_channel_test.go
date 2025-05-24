package tests

import (
	"testing"
	"time"

	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/output"
	libmeshsync "github.com/layer5io/meshsync/pkg/lib/meshsync"
	"github.com/stretchr/testify/assert"
)

var k8sClusterMeshsyncLibraryTestCasesChannelModeData []k8sClusterMeshsyncLibraryTestCaseStruct = []k8sClusterMeshsyncLibraryTestCaseStruct{
	{
		name: "output mode channel: number of messages received from meshsync is greater than zero",
		meshsyncRunOptions: []libmeshsync.OptionsSetter{
			libmeshsync.WithOutputMode(config.OutputModeChannel),
			libmeshsync.WithStopAfterDuration(8 * time.Second),
		},
		channelMessageHandler: func(
			t *testing.T,
			out chan *output.ChannelItem,
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
}
