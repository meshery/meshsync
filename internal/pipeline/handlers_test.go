package pipeline

import (
	"testing"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/logger"
	internalconfig "github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

// recordingWriter is a test double for output.Writer that records every
// resource handed to it, so tests can assert what the delete handler published.
type recordingWriter struct {
	written []model.KubernetesResource
	events  []broker.EventType
}

func (w *recordingWriter) Write(obj model.KubernetesResource, evtype broker.EventType, _ internalconfig.PipelineConfig) error {
	w.written = append(w.written, obj)
	w.events = append(w.events, evtype)
	return nil
}

// newTestRegisterInformer builds a RegisterInformer wired to a recordingWriter
// with the DELETE event enabled - the minimum needed to drive DeleteFunc end to
// end. The logger uses zero-value Options, which defaults to logrus PanicLevel
// and therefore stays silent during the test.
func newTestRegisterInformer(t *testing.T) (*RegisterInformer, *recordingWriter) {
	t.Helper()

	log, err := logger.New("meshsync-test", logger.Options{})
	require.NoError(t, err)

	writer := &recordingWriter{}
	ri := &RegisterInformer{
		log:          log,
		outputWriter: writer,
		config: internalconfig.PipelineConfig{
			Name:   "pods.v1.",
			Events: []string{string(broker.Delete)},
		},
		clusterID: "test-cluster",
	}
	return ri, writer
}

func newUnstructuredPod(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

// newUpdateTestRegisterInformer mirrors newTestRegisterInformer but enables the
// UPDATE event so UpdateFunc actually reaches the writer.
func newUpdateTestRegisterInformer(t *testing.T) (*RegisterInformer, *recordingWriter) {
	t.Helper()

	log, err := logger.New("meshsync-test", logger.Options{})
	require.NoError(t, err)

	writer := &recordingWriter{}
	ri := &RegisterInformer{
		log:          log,
		outputWriter: writer,
		config: internalconfig.PipelineConfig{
			Name:   "pods.v1.",
			Events: []string{string(broker.Update)},
		},
		clusterID: "test-cluster",
	}
	return ri, writer
}

func newUnstructuredPodWithRV(name, namespace, resourceVersion string) *unstructured.Unstructured {
	pod := newUnstructuredPod(name, namespace)
	pod.SetResourceVersion(resourceVersion)
	return pod
}

// TestUpdateFunc_SuppressesEqualResourceVersion verifies that a resync-style
// UPDATE, where the resourceVersion is unchanged, is treated as a no-op and
// nothing is published.
func TestUpdateFunc_SuppressesEqualResourceVersion(t *testing.T) {
	ri, writer := newUpdateTestRegisterInformer(t)
	handlers := ri.GetEventHandlers()

	old := newUnstructuredPodWithRV("pod", "default", "12345")
	updated := newUnstructuredPodWithRV("pod", "default", "12345")

	handlers.UpdateFunc(old, updated)

	assert.Empty(t, writer.written, "an unchanged resourceVersion must not publish an UPDATE")
}

// TestUpdateFunc_PublishesChangedResourceVersion verifies that a real change,
// where the resourceVersion differs, is published.
func TestUpdateFunc_PublishesChangedResourceVersion(t *testing.T) {
	ri, writer := newUpdateTestRegisterInformer(t)
	handlers := ri.GetEventHandlers()

	old := newUnstructuredPodWithRV("pod", "default", "12345")
	updated := newUnstructuredPodWithRV("pod", "default", "12346")

	handlers.UpdateFunc(old, updated)

	require.Len(t, writer.written, 1, "a changed resourceVersion must publish exactly one UPDATE")
	assert.Equal(t, broker.Update, writer.events[0])
}

// TestUpdateFunc_OpaqueResourceVersion is the regression test for treating
// resourceVersion as an opaque string. A numeric parse would map both of these
// non-numeric versions to 0, wrongly suppressing a genuine change; comparing the
// raw strings publishes the update as it must.
func TestUpdateFunc_OpaqueResourceVersion(t *testing.T) {
	ri, writer := newUpdateTestRegisterInformer(t)
	handlers := ri.GetEventHandlers()

	old := newUnstructuredPodWithRV("pod", "default", "W/abc")
	updated := newUnstructuredPodWithRV("pod", "default", "W/def")

	handlers.UpdateFunc(old, updated)

	require.Len(t, writer.written, 1, "differing opaque resourceVersions must publish an UPDATE")
	assert.Equal(t, broker.Update, writer.events[0])
}

// TestDeleteFunc_Tombstone is the regression test for the panic that occurred
// when the informer delivered a cache.DeletedFinalStateUnknown tombstone - the
// exact stale-delete case that happens after a watch gap/resync. The handler
// must unwrap the tombstone and publish the wrapped object rather than panicking
// on an unchecked type assertion.
func TestDeleteFunc_Tombstone(t *testing.T) {
	ri, writer := newTestRegisterInformer(t)
	handlers := ri.GetEventHandlers()

	tombstone := cache.DeletedFinalStateUnknown{
		Key: "default/tombstoned-pod",
		Obj: newUnstructuredPod("tombstoned-pod", "default"),
	}

	assert.NotPanics(t, func() {
		handlers.DeleteFunc(tombstone)
	}, "DeleteFunc must not panic on a cache.DeletedFinalStateUnknown tombstone")

	require.Len(t, writer.written, 1, "the wrapped object should be published exactly once")
	assert.Equal(t, broker.Delete, writer.events[0])
	assert.Equal(t, "Pod", writer.written[0].Kind)
	assert.Equal(t, "test-cluster", writer.written[0].ClusterID)
	require.NotNil(t, writer.written[0].KubernetesResourceMeta)
	assert.Equal(t, "tombstoned-pod", writer.written[0].KubernetesResourceMeta.Name)
}

// TestDeleteFunc_Unstructured covers the ordinary path where the informer
// delivers the object directly as *unstructured.Unstructured.
func TestDeleteFunc_Unstructured(t *testing.T) {
	ri, writer := newTestRegisterInformer(t)
	handlers := ri.GetEventHandlers()

	assert.NotPanics(t, func() {
		handlers.DeleteFunc(newUnstructuredPod("live-pod", "kube-system"))
	})

	require.Len(t, writer.written, 1)
	assert.Equal(t, broker.Delete, writer.events[0])
	require.NotNil(t, writer.written[0].KubernetesResourceMeta)
	assert.Equal(t, "live-pod", writer.written[0].KubernetesResourceMeta.Name)
}

// TestDeleteFunc_UnexpectedType ensures the handler degrades gracefully - it
// must neither panic nor publish anything - when it receives neither an
// *unstructured.Unstructured nor a tombstone wrapping one.
func TestDeleteFunc_UnexpectedType(t *testing.T) {
	cases := []struct {
		name string
		obj  interface{}
	}{
		{name: "raw unexpected type", obj: "not-an-object"},
		{name: "tombstone wrapping unexpected type", obj: cache.DeletedFinalStateUnknown{Key: "x", Obj: "not-an-object"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ri, writer := newTestRegisterInformer(t)
			handlers := ri.GetEventHandlers()

			assert.NotPanics(t, func() {
				handlers.DeleteFunc(tc.obj)
			})
			assert.Empty(t, writer.written, "nothing should be published for an unhandled object type")
		})
	}
}
