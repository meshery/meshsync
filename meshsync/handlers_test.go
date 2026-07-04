package meshsync

import (
	"reflect"
	"testing"
	"time"

	configprovider "github.com/meshery/meshkit/config/provider"
	"github.com/meshery/meshkit/logger"
	"github.com/meshery/meshsync/internal/channels"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

// TestSplitIntoMultipleSlices tests the splitIntoMultipleSlices function
// by providing different input test cases and comparing the output with the expected output.
func TestSplitIntoMultipleSlices(t *testing.T) {
	testCases := []struct {
		name            string
		input           []model.KubernetesResource
		maxItmsPerSlice int
		expectedOutput  [][]model.KubernetesResource
	}{
		{
			name:            "test with 0 items",
			input:           []model.KubernetesResource{},
			maxItmsPerSlice: 10,
			expectedOutput:  [][]model.KubernetesResource{},
		},

		{
			name: "test with 1 item",
			input: []model.KubernetesResource{
				{
					Kind: "test",
				},
			},
			maxItmsPerSlice: 10,
			expectedOutput: [][]model.KubernetesResource{
				{
					{
						Kind: "test",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := splitIntoMultipleSlices(tc.input, tc.maxItmsPerSlice)
			if !reflect.DeepEqual(output, tc.expectedOutput) {
				t.Errorf("expected %v, but got %v", tc.expectedOutput, output)
			}
		})
	}
}

func newConfigTestHandler(t *testing.T, seed config.PipelineConfigs) *Handler {
	t.Helper()
	cfg, err := configprovider.NewInMem(configprovider.Options{})
	if err != nil {
		t.Fatalf("failed to create in-mem config: %v", err)
	}
	if err := cfg.SetObject(config.ResourcesKey, map[string]config.PipelineConfigs{
		config.GlobalResourceKey: seed,
	}); err != nil {
		t.Fatalf("failed to seed resources: %v", err)
	}
	return &Handler{Config: cfg}
}

func globalPipelines(t *testing.T, h *Handler) config.PipelineConfigs {
	t.Helper()
	existing := make(map[string]config.PipelineConfigs)
	if err := h.Config.GetObject(config.ResourcesKey, &existing); err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	return existing[config.GlobalResourceKey]
}

// Shared fixtures for the CRD-watch tests. The values mirror a cert-manager CRD -
// the controller whose continuous MODIFIED events motivated the resync fix.
const (
	testCRDGroup    = "cert-manager.io"
	testCRDVersion  = "v1"
	testCRDPlural   = "certificates"
	testCRDPipeline = "certificates.v1.cert-manager.io" // resource.version.group
)

// crdPipelineEvents mirrors the broker event set updatePipelineConfig registers
// for a discovered CRD (broker.Add/Update/Delete), pinned as literals so the
// tests fail if that wire contract ever drifts.
var crdPipelineEvents = []string{"ADDED", "MODIFIED", "DELETED"}

func testCRDGVR() *schema.GroupVersionResource {
	return &schema.GroupVersionResource{Group: testCRDGroup, Version: testCRDVersion, Resource: testCRDPlural}
}

// TestUpdatePipelineConfig verifies that a CRD watch event is only reported as a
// config change (and therefore only triggers an informer resync) when it
// actually mutates the set of watched resources. In particular MODIFIED events -
// which controllers like cert-manager's cainjector emit continuously - must be
// no-ops.
func TestUpdatePipelineConfig(t *testing.T) {
	gvr := testCRDGVR()
	existingSeed := func() config.PipelineConfigs {
		return config.PipelineConfigs{{
			Name:      testCRDPipeline,
			PublishTo: config.DefaultPublishingSubject,
			Events:    crdPipelineEvents,
		}}
	}

	tests := []struct {
		name        string
		seed        config.PipelineConfigs
		eventType   watch.EventType
		wantChanged bool
		wantLen     int
	}{
		{"added new resource registers it", config.PipelineConfigs{}, watch.Added, true, 1},
		{"added already-watched resource is a no-op", existingSeed(), watch.Added, false, 1},
		{"deleted watched resource removes it", existingSeed(), watch.Deleted, true, 0},
		{"deleted unwatched resource is a no-op", config.PipelineConfigs{}, watch.Deleted, false, 0},
		{"modified watched resource never resyncs", existingSeed(), watch.Modified, false, 1},
		{"modified unwatched resource never registers", config.PipelineConfigs{}, watch.Modified, false, 0},
		{"bookmark never changes config", existingSeed(), watch.Bookmark, false, 1},
		{"error never changes config", existingSeed(), watch.Error, false, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newConfigTestHandler(t, tc.seed)
			changed, err := h.updatePipelineConfig(tc.eventType, gvr)
			if err != nil {
				t.Fatalf("updatePipelineConfig returned error: %v", err)
			}
			if changed != tc.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tc.wantChanged)
			}
			if got := globalPipelines(t, h); len(got) != tc.wantLen {
				t.Errorf("global pipelines len = %d, want %d (%v)", len(got), tc.wantLen, got)
			}
		})
	}
}

// TestUpdatePipelineConfigIdempotentAdd guards against duplicate pipeline
// configs: a re-established CRD watch re-lists every existing CRD as ADDED, and
// registering unconditionally would both double-publish resources and force a
// needless resync each time.
func TestUpdatePipelineConfigIdempotentAdd(t *testing.T) {
	gvr := testCRDGVR()
	h := newConfigTestHandler(t, config.PipelineConfigs{})

	changed, err := h.updatePipelineConfig(watch.Added, gvr)
	if err != nil || !changed {
		t.Fatalf("first Added: changed=%v err=%v, want changed=true, err=nil", changed, err)
	}

	changed, err = h.updatePipelineConfig(watch.Added, gvr)
	if err != nil {
		t.Fatalf("second Added returned error: %v", err)
	}
	if changed {
		t.Error("second Added reported changed=true; a re-listed CRD must be a no-op")
	}
	if got := globalPipelines(t, h); len(got) != 1 {
		t.Errorf("expected exactly 1 pipeline config after duplicate ADD, got %d: %v", len(got), got)
	}
}

// TestUpdatePipelineConfigRegistersBrokerEvents locks the wire contract: a newly
// discovered CRD must be registered with exactly the broker event strings the
// pipeline filters on, so publishing is not silently dropped if the broker
// EventType values ever drift.
func TestUpdatePipelineConfigRegistersBrokerEvents(t *testing.T) {
	gvr := testCRDGVR()
	h := newConfigTestHandler(t, config.PipelineConfigs{})

	if _, err := h.updatePipelineConfig(watch.Added, gvr); err != nil {
		t.Fatalf("updatePipelineConfig: %v", err)
	}
	got := globalPipelines(t, h)
	if len(got) != 1 {
		t.Fatalf("expected 1 registered pipeline, got %d: %v", len(got), got)
	}
	if !reflect.DeepEqual(got[0].Events, crdPipelineEvents) {
		t.Errorf("registered events = %v, want %v", got[0].Events, crdPipelineEvents)
	}
}

// logInfoLevel is MeshKit's info log level. logger.Options.LogLevel is a plain
// int mirroring logrus severity levels (info == 4); it is spelled out here so
// the tests depend only on MeshKit for logging, not on logrus directly.
const logInfoLevel = 4

func testLogger(t *testing.T) logger.Handler {
	t.Helper()
	log, err := logger.New("meshsync-test", logger.Options{
		Format:   logger.SyslogLogFormat,
		LogLevel: logInfoLevel,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	return log
}

// crdEvent builds a CustomResourceDefinition watch event equivalent to what the
// API server streams, so it exercises the real parseCRDEvent/GVR-extraction path.
func crdEvent(eventType watch.EventType, group, plural, version string) watch.Event {
	return watch.Event{
		Type: eventType,
		Object: &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]interface{}{"name": plural + "." + group},
			"spec": map[string]interface{}{
				"group":    group,
				"names":    map[string]interface{}{"plural": plural},
				"versions": []interface{}{map[string]interface{}{"name": version}},
			},
		}},
	}
}

// resyncSignaled runs handleCRDEvent and reports whether it requested an informer
// resync (a blocking send on the ReSync channel).
func resyncSignaled(t *testing.T, h *Handler, event watch.Event) bool {
	t.Helper()
	done := make(chan struct{})
	go func() {
		h.handleCRDEvent(event)
		close(done)
	}()

	resyncCh := h.channelPool[channels.ReSync].(channels.ReSyncChannel)
	select {
	case <-resyncCh:
		<-done // let the (now unblocked) ReSyncInformer send return
		return true
	case <-done:
		return false
	case <-time.After(2 * time.Second):
		t.Fatal("handleCRDEvent did not complete in time")
		return false
	}
}

// TestHandleCRDEventResync is an end-to-end check of the resync trigger. It is
// the regression guard for the cert-manager re-list storm: a MODIFIED CRD event
// must not tear down and rebuild every informer, while a genuinely new CRD still
// must.
func TestHandleCRDEventResync(t *testing.T) {
	watched := config.PipelineConfigs{{
		Name:      testCRDPipeline,
		PublishTo: config.DefaultPublishingSubject,
		Events:    crdPipelineEvents,
	}}

	tests := []struct {
		name       string
		seed       config.PipelineConfigs
		event      watch.Event
		wantResync bool
	}{
		{
			name:       "modified watched crd does not resync",
			seed:       watched,
			event:      crdEvent(watch.Modified, testCRDGroup, testCRDPlural, testCRDVersion),
			wantResync: false,
		},
		{
			name:       "re-listed added crd does not resync",
			seed:       watched,
			event:      crdEvent(watch.Added, testCRDGroup, testCRDPlural, testCRDVersion),
			wantResync: false,
		},
		{
			name:       "newly added crd resyncs",
			seed:       config.PipelineConfigs{},
			event:      crdEvent(watch.Added, testCRDGroup, testCRDPlural, testCRDVersion),
			wantResync: true,
		},
		{
			name:       "deleted watched crd resyncs",
			seed:       watched,
			event:      crdEvent(watch.Deleted, testCRDGroup, testCRDPlural, testCRDVersion),
			wantResync: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newConfigTestHandler(t, tc.seed)
			h.Log = testLogger(t)
			h.channelPool = map[string]channels.GenericChannel{
				channels.ReSync: channels.NewReSyncChannel(),
			}
			if got := resyncSignaled(t, h, tc.event); got != tc.wantResync {
				t.Errorf("resync signaled = %v, want %v", got, tc.wantResync)
			}
		})
	}
}

// TestJitterBounds checks the backoff jitter stays within [d/2, d] so retries
// are neither instantaneous nor longer than the intended backoff.
func TestJitterBounds(t *testing.T) {
	for _, d := range []time.Duration{2 * time.Second, 30 * time.Second, 2 * time.Minute} {
		for i := 0; i < 1000; i++ {
			j := jitter(d)
			if j < d/2 || j > d {
				t.Fatalf("jitter(%s) = %s, want within [%s, %s]", d, j, d/2, d)
			}
		}
	}
	if j := jitter(0); j != 0 {
		t.Errorf("jitter(0) = %s, want 0", j)
	}
	if j := jitter(-time.Second); j != 0 {
		t.Errorf("jitter(negative) = %s, want 0", j)
	}
}
