package tests

import (
	"testing"
	"time"

	"github.com/layer5io/meshkit/broker"
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

var defaultClusterTestCasesData []defaultClusterTestCaseStruct

func init() {
	for _, tcs := range [][]defaultClusterTestCaseStruct{
		defaultClusterTestCasesNatsModeData,
		defaultClusterTestCasesFileModeData,
		defaultClusterTestCasesChannelModeData,
	} {
		defaultClusterTestCasesData = append(defaultClusterTestCasesData, tcs...)
	}
}
