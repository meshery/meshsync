package pipeline

import (
	"testing"

	internalconfig "github.com/meshery/meshsync/internal/config"
)

// TestNewDoesNotAccumulateSteps guards against the discovery stages being shared
// mutable state. New is called once per discovery, and again on every resync, so
// each call must produce an independent pipeline. If stage state persisted across
// calls, the stages would accumulate duplicate steps that hold stale informer
// factories and closed stop channels from prior runs.
func TestNewDoesNotAccumulateSteps(t *testing.T) {
	log := newTestLogger(t)
	// Empty configs mean no RegisterInformer steps, so a correct pipeline holds
	// exactly one step: the single StartInformers step.
	plConfigs := map[string]internalconfig.PipelineConfigs{}

	totalSteps := func() int {
		factory, _ := newSeededFactory(t)
		stopChan := make(chan struct{})
		pl := New(log, factory, nil, plConfigs, stopChan, "", internalconfig.OutputFiltrationContainer{})
		total := 0
		for _, st := range pl.Stages {
			total += len(st.Steps)
		}
		return total
	}

	if first := totalSteps(); first != 1 {
		t.Fatalf("first New() produced %d steps, want 1", first)
	}
	if second := totalSteps(); second != 1 {
		t.Fatalf("steps accumulated across New() calls: got %d on the second call, want 1", second)
	}
}
