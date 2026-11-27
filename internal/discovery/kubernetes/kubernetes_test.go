package kubernetes

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	configprovider "github.com/meshery/meshkit/config/provider"
	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/logger"
	mesherykube "github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/meshsync/internal/channels"
	internalconfig "github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/internal/discovery"
	"github.com/meshery/meshsync/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

var podsGVR = schema.GroupVersionResource{Version: "v1", Resource: "pods"}

func newPod(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":            name,
				"namespace":       "default",
				"uid":             name + "-uid",
				"resourceVersion": "1",
				"labels":          map[string]interface{}{"app": name},
			},
		},
	}
}

// recordingEmit collects every DiscoveredResource handed to it.
type recordingEmit struct {
	mu        sync.Mutex
	resources []model.DiscoveredResource
	events    []broker.EventType
}

func (r *recordingEmit) emit(res model.DiscoveredResource, evtype broker.EventType, _ internalconfig.PipelineConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resources = append(r.resources, res)
	r.events = append(r.events, evtype)
	return nil
}

func (r *recordingEmit) snapshot() ([]model.DiscoveredResource, []broker.EventType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]model.DiscoveredResource(nil), r.resources...), append([]broker.EventType(nil), r.events...)
}

func newTestDiscoverer(t *testing.T, objects ...runtime.Object) (*Discoverer, channels.StopChannel) {
	t.Helper()

	log, err := logger.New("meshsync-test", logger.Options{})
	require.NoError(t, err)

	cfg, err := configprovider.NewInMem(configprovider.Options{})
	require.NoError(t, err)
	require.NoError(t, cfg.SetObject(internalconfig.ResourcesKey, map[string]internalconfig.PipelineConfigs{
		internalconfig.GlobalResourceKey: {{
			Name:      "pods.v1.",
			PublishTo: "meshery.meshsync.core",
			Events:    []string{string(broker.Add), string(broker.Update), string(broker.Delete)},
		}},
	}))

	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{podsGVR: "PodList"},
		objects...,
	)

	stopCh := channels.NewStopChannel()
	d, err := New(Options{
		Config:     cfg,
		KubeClient: &mesherykube.Client{DynamicKubeClient: dynamicClient},
		Log:        log,
		ChannelPool: map[string]channels.GenericChannel{
			channels.Stop:   stopCh,
			channels.ReSync: channels.NewReSyncChannel(),
		},
	})
	require.NoError(t, err)
	return d, stopCh
}

func TestDiscovererIdentity(t *testing.T) {
	d, _ := newTestDiscoverer(t)

	assert.Equal(t, model.PlatformKubernetes, d.Platform())

	// No kube client: GetClusterID yields "", the same legacy cluster_id value.
	scopeID, err := d.ScopeID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "", scopeID)
}

func TestDiscoverIsNotSupported(t *testing.T) {
	d, _ := newTestDiscoverer(t)

	err := d.Discover(context.Background(), func(model.DiscoveredResource, broker.EventType, internalconfig.PipelineConfig) error {
		t.Fatal("Discover must not emit")
		return nil
	})

	require.Error(t, err)
	assert.Equal(t, discovery.ErrDiscoverNotSupportedCode, meshkiterrors.GetCode(err))
}

// TestWatchEmitsUnchangedKubernetesPayload is the Phase 0 regression guard: the
// Kubernetes Discoverer must emit, for every informer event, exactly the
// KubernetesResource that model.ParseList produced before the Discoverer
// existed, wrapped in an envelope whose ScopeID matches its cluster_id.
func TestWatchEmitsUnchangedKubernetesPayload(t *testing.T) {
	pod := newPod("web")
	d, stopCh := newTestDiscoverer(t, pod)
	rec := &recordingEmit{}

	watchDone := make(chan error, 1)
	go func() { watchDone <- d.Watch(context.Background(), stopCh, rec.emit) }()

	require.Eventually(t, func() bool {
		res, _ := rec.snapshot()
		return len(res) == 1
	}, 5*time.Second, 10*time.Millisecond, "initial list must emit the existing pod")

	resources, events := rec.snapshot()
	assert.Equal(t, broker.Add, events[0])

	got := resources[0]
	assert.Equal(t, model.PlatformKubernetes, got.Platform)
	assert.Equal(t, "Pod", got.Kind)
	assert.Equal(t, "web", got.Name)
	assert.Equal(t, map[string]string{"app": "web"}, got.Labels)
	require.NotNil(t, got.Kubernetes)
	assert.Equal(t, got.ScopeID, got.Kubernetes.ClusterID)

	want := model.ParseList(*pod, broker.Add, "")
	assert.Equal(t, want, *got.Kubernetes, "Kubernetes payload must match the pre-refactor ParseList output")

	// Against the fake dynamic client, Handler.Run's existing teardown completes
	// about 5s after discovery starts (the same on master without the
	// Discoverer), so allow well beyond that before calling it a hang.
	close(stopCh)
	select {
	case err := <-watchDone:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatal("Watch did not return after the Stop channel closed")
	}

	// Shutdown after Watch has returned must be safe, and safe to repeat.
	d.Shutdown()
	d.Shutdown()
}
