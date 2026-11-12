package meshsync

import (
	"reflect"
	"sync"
	"sync/atomic"
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

// TestRun_DebounceCoalescesBurst verifies that rapid bursts of ReSync signals within
// the debounce window coalesce into a single restart.
func TestRun_DebounceCoalescesBurst(t *testing.T) {
	h := newConfigTestHandler(t, config.PipelineConfigs{})
	h.Log = testLogger(t)
	h.DebounceDuration = 150 * time.Millisecond

	stopCh := channels.NewStopChannel()
	resyncCh := channels.NewReSyncChannel()
	h.channelPool = map[string]channels.GenericChannel{
		channels.Stop:   stopCh,
		channels.ReSync: resyncCh,
	}

	var discoveryStarts atomic.Int32
	var informerUpdates atomic.Int32

	h.DiscoveryStarter = func(pipelineCh chan struct{}) {
		discoveryStarts.Add(1)
		<-pipelineCh
	}
	h.InformerUpdater = func() error {
		informerUpdates.Add(1)
		return nil
	}

	runDone := make(chan struct{})
	go func() {
		h.Run()
		close(runDone)
	}()

	// Wait for initial discovery to start
	for i := 0; i < 50; i++ {
		if discoveryStarts.Load() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := discoveryStarts.Load(); got != 1 {
		t.Fatalf("expected initial discovery to start, got %d", got)
	}

	// Send burst of 4 ReSync signals 10ms apart (total 30ms span, 5x margin
	// against the 150ms debounce window for slow CI runners)
	for i := 0; i < 4; i++ {
		resyncCh <- struct{}{}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait past the debounce duration (fires ~180ms after the first signal)
	time.Sleep(300 * time.Millisecond)

	// Exactly 1 restart should have occurred (total discoveryStarts == 2)
	if got := discoveryStarts.Load(); got != 2 {
		t.Errorf("expected 2 total discoveries (1 initial + 1 debounced restart), got %d", got)
	}
	if got := informerUpdates.Load(); got != 1 {
		t.Errorf("expected 1 informer update, got %d", got)
	}

	// Clean shutdown
	close(stopCh)
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after Stop was closed")
	}
}

// TestRun_StopCancelsPendingDebounce verifies that if channels.Stop is closed while
// a debounce timer is pending, the timer is aborted immediately and no late
// discovery or informer update occurs after Run() returns.
func TestRun_StopCancelsPendingDebounce(t *testing.T) {
	h := newConfigTestHandler(t, config.PipelineConfigs{})
	h.Log = testLogger(t)
	h.DebounceDuration = 100 * time.Millisecond

	stopCh := channels.NewStopChannel()
	resyncCh := channels.NewReSyncChannel()
	h.channelPool = map[string]channels.GenericChannel{
		channels.Stop:   stopCh,
		channels.ReSync: resyncCh,
	}

	var discoveryStarts atomic.Int32
	var informerUpdates atomic.Int32

	h.DiscoveryStarter = func(pipelineCh chan struct{}) {
		discoveryStarts.Add(1)
		<-pipelineCh
	}
	h.InformerUpdater = func() error {
		informerUpdates.Add(1)
		return nil
	}

	runDone := make(chan struct{})
	go func() {
		h.Run()
		close(runDone)
	}()

	// Wait for initial discovery to start
	for i := 0; i < 50; i++ {
		if discoveryStarts.Load() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := discoveryStarts.Load(); got != 1 {
		t.Fatalf("expected initial discovery to start, got %d", got)
	}

	// Trigger a resync, scheduling a restart in 100ms
	resyncCh <- struct{}{}

	// Stop 25ms later, well before the 100ms debounce fires
	time.Sleep(25 * time.Millisecond)
	close(stopCh)

	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after Stop was closed")
	}

	// Snapshot counts right at shutdown
	shutdownDiscoveries := discoveryStarts.Load()
	shutdownInformerUpdates := informerUpdates.Load()

	// Wait past the original 100ms debounce window
	time.Sleep(150 * time.Millisecond)

	// Ensure no late timer fired after shutdown
	if got := discoveryStarts.Load(); got != shutdownDiscoveries {
		t.Errorf("zombie discovery started post-shutdown: expected %d, got %d", shutdownDiscoveries, got)
	}
	if got := informerUpdates.Load(); got != shutdownInformerUpdates {
		t.Errorf("zombie informer update post-shutdown: expected %d, got %d", shutdownInformerUpdates, got)
	}
}

// TestRun_MultipleHandlersIndependent verifies that two independent Handler instances
// do not share or overwrite debounce timers or mutexes (confirming removal of package globals).
func TestRun_MultipleHandlersIndependent(t *testing.T) {
	h1 := newConfigTestHandler(t, config.PipelineConfigs{})
	h1.Log = testLogger(t)
	h1.DebounceDuration = 30 * time.Millisecond

	stopCh1 := channels.NewStopChannel()
	resyncCh1 := channels.NewReSyncChannel()
	h1.channelPool = map[string]channels.GenericChannel{
		channels.Stop:   stopCh1,
		channels.ReSync: resyncCh1,
	}

	h2 := newConfigTestHandler(t, config.PipelineConfigs{})
	h2.Log = testLogger(t)
	h2.DebounceDuration = 30 * time.Millisecond

	stopCh2 := channels.NewStopChannel()
	resyncCh2 := channels.NewReSyncChannel()
	h2.channelPool = map[string]channels.GenericChannel{
		channels.Stop:   stopCh2,
		channels.ReSync: resyncCh2,
	}

	var d1Starts, d2Starts atomic.Int32

	h1.DiscoveryStarter = func(pipelineCh chan struct{}) {
		d1Starts.Add(1)
		<-pipelineCh
	}
	h1.InformerUpdater = func() error {
		return nil
	}
	h2.DiscoveryStarter = func(pipelineCh chan struct{}) {
		d2Starts.Add(1)
		<-pipelineCh
	}
	h2.InformerUpdater = func() error {
		return nil
	}

	runDone1 := make(chan struct{})
	go func() {
		h1.Run()
		close(runDone1)
	}()

	runDone2 := make(chan struct{})
	go func() {
		h2.Run()
		close(runDone2)
	}()

	// Wait for both to initialize initial discovery
	for i := 0; i < 50; i++ {
		if d1Starts.Load() == 1 && d2Starts.Load() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Trigger resync ONLY on h1
	resyncCh1 <- struct{}{}

	// Wait for h1 to debounce and restart (30ms + margin)
	time.Sleep(80 * time.Millisecond)

	if got := d1Starts.Load(); got != 2 {
		t.Errorf("expected h1 to have restarted (2 starts), got %d", got)
	}
	if got := d2Starts.Load(); got != 1 {
		t.Errorf("expected h2 to remain untouched (1 start), got %d", got)
	}

	close(stopCh1)
	close(stopCh2)

	select {
	case <-runDone1:
	case <-time.After(2 * time.Second):
		t.Fatal("h1.Run() did not exit after Stop was closed")
	}

	select {
	case <-runDone2:
	case <-time.After(2 * time.Second):
		t.Fatal("h2.Run() did not exit after Stop was closed")
	}
}

// TestRun_ReSyncSendDoesNotBlockOnStoppedHandler verifies that handleResyncRequest
// and handleCRDEvent do not block if Stop has already been signaled.
func TestRun_ReSyncSendDoesNotBlockOnStoppedHandler(t *testing.T) {
	h := newConfigTestHandler(t, config.PipelineConfigs{})
	h.Log = testLogger(t)

	stopCh := channels.NewStopChannel()
	close(stopCh) // already stopped

	resyncCh := channels.NewReSyncChannel()
	h.channelPool = map[string]channels.GenericChannel{
		channels.Stop:   stopCh,
		channels.ReSync: resyncCh,
	}

	// handleResyncRequest must return without blocking
	done := make(chan struct{})
	go func() {
		h.handleResyncRequest()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handleResyncRequest blocked on stopped handler")
	}

	// handleCRDEvent with resync-triggering event must also return without blocking
	crdDone := make(chan struct{})
	go func() {
		h.handleCRDEvent(crdEvent(watch.Added, testCRDGroup, testCRDPlural, testCRDVersion))
		close(crdDone)
	}()

	select {
	case <-crdDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handleCRDEvent blocked on stopped handler")
	}
}

// TestRun_ConcurrentReSyncRace verifies that rapid concurrent ReSync calls under -race
// do not cause data races or WaitGroup panics.
func TestRun_ConcurrentReSyncRace(t *testing.T) {
	h := newConfigTestHandler(t, config.PipelineConfigs{})
	h.Log = testLogger(t)
	h.DebounceDuration = 20 * time.Millisecond

	stopCh := channels.NewStopChannel()
	resyncCh := channels.NewReSyncChannel()
	h.channelPool = map[string]channels.GenericChannel{
		channels.Stop:   stopCh,
		channels.ReSync: resyncCh,
	}

	h.DiscoveryStarter = func(pipelineCh chan struct{}) {
		<-pipelineCh
	}
	h.InformerUpdater = func() error {
		return nil
	}

	runDone := make(chan struct{})
	go func() {
		h.Run()
		close(runDone)
	}()

	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				h.handleResyncRequest()
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	time.Sleep(60 * time.Millisecond)

	close(stopCh)
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after Stop was closed")
	}
}
