package pipeline

import (
	"testing"
	"time"

	"github.com/meshery/meshkit/logger"
	"github.com/myntra/pipeline"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
)

var widgetGVR = schema.GroupVersionResource{Group: "meshsync.test", Version: "v1", Resource: "widgets"}

func newTestLogger(t *testing.T) logger.Handler {
	t.Helper()
	// Quiet level: the step logs cache-sync status at Debug, which we do not want
	// polluting test output.
	log, err := logger.New("meshsync-test", logger.Options{
		Format:   logger.JsonLogFormat,
		LogLevel: int(logrus.ErrorLevel),
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	return log
}

func newWidget(name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "meshsync.test", Version: "v1", Kind: "Widget"})
	u.SetNamespace("default")
	u.SetName(name)
	return u
}

// newSeededFactory builds a factory over a fake dynamic client pre-loaded with
// objs and registers the informer for widgetGVR. Registration mirrors what
// RegisterInformer.Exec does in the earlier discovery stages, so the factory is
// in the same state StartInformers sees in production.
func newSeededFactory(t *testing.T, objs ...*unstructured.Unstructured) (dynamicinformer.DynamicSharedInformerFactory, cache.Store) {
	t.Helper()
	runtimeObjs := make([]runtime.Object, len(objs))
	for i, o := range objs {
		runtimeObjs[i] = o
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{widgetGVR: "WidgetList"},
		runtimeObjs...,
	)
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, 0, metav1.NamespaceAll, nil)
	store := factory.ForResource(widgetGVR).Informer().GetStore()
	return factory, store
}

// TestStartInformersExec_PrimesCache is the regression guard for the ordering
// bug: WaitForCacheSync must run after Start. With the correct order the started
// informers' caches are guaranteed primed by the time Exec returns. (With the
// old Wait-before-Start order, WaitForCacheSync was a no-op and the store would
// still be empty here.)
func TestStartInformersExec_PrimesCache(t *testing.T) {
	factory, store := newSeededFactory(t, newWidget("w1"), newWidget("w2"))
	stopChan := make(chan struct{})
	defer close(stopChan)

	step := newStartInformersStep(stopChan, newTestLogger(t), factory)

	inData := map[string]cache.Store{widgetGVR.Resource: store}
	res := step.Exec(&pipeline.Request{Data: inData})

	if res.Error != nil {
		t.Fatalf("Exec returned error: %v", res.Error)
	}
	if got := len(store.List()); got != 2 {
		t.Fatalf("cache not primed synchronously: store holds %d objects, want 2", got)
	}
	// StartInformers is the final stage; it must pass the accumulated store map
	// from earlier stages through untouched (discovery.go relies on this).
	outData, ok := res.Data.(map[string]cache.Store)
	if !ok || outData[widgetGVR.Resource] == nil {
		t.Fatalf("Exec dropped the accumulated store map: got %#v", res.Data)
	}
}

// TestStartInformersExec_ClosedStopChan verifies the teardown path: when
// stopChan is already closed (shutdown or resync), WaitForCacheSync must unblock
// so Exec returns promptly instead of hanging, and it must not surface an error.
func TestStartInformersExec_ClosedStopChan(t *testing.T) {
	factory, _ := newSeededFactory(t, newWidget("w1"))
	stopChan := make(chan struct{})
	close(stopChan)

	step := newStartInformersStep(stopChan, newTestLogger(t), factory)

	done := make(chan *pipeline.Result, 1)
	go func() { done <- step.Exec(&pipeline.Request{}) }()

	select {
	case res := <-done:
		if res.Error != nil {
			t.Fatalf("Exec returned error on closed stopChan: %v", res.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Exec blocked on a closed stopChan; WaitForCacheSync must unblock on stop")
	}
}
