package output

import (
	"fmt"
	"sync"
	"testing"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	"github.com/stretchr/testify/assert"
)

// resourceWithUID builds a resource carrying the given UID. resourceVersion is
// folded into the payload so callers can produce two values with the same UID
// but different serialized content.
func resourceWithUID(uid, resourceVersion string) model.KubernetesResource {
	return model.KubernetesResource{
		Kind: "Pod",
		KubernetesResourceMeta: &model.KubernetesResourceObjectMeta{
			UID:             uid,
			ResourceVersion: resourceVersion,
		},
	}
}

func TestContentDeduplicatorWriter_SkipsByteIdentical(t *testing.T) {
	mock := &mockWriter{}
	writer := NewContentDeduplicatorWriter(mock)
	cfg := config.PipelineConfig{}

	res := resourceWithUID("uid-1", "100")

	// First ADD publishes; second identical UPDATE is byte-identical and skipped.
	assert.NoError(t, writer.Write(res, broker.Add, cfg))
	assert.NoError(t, writer.Write(res, broker.Update, cfg))
	assert.NoError(t, writer.Write(res, broker.Update, cfg))

	assert.Len(t, mock.written, 1)
	assert.Equal(t, broker.Add, mock.events[0])
}

func TestContentDeduplicatorWriter_RepublishesOnChange(t *testing.T) {
	mock := &mockWriter{}
	writer := NewContentDeduplicatorWriter(mock)
	cfg := config.PipelineConfig{}

	// Same UID, changing content each time: every write must be forwarded.
	assert.NoError(t, writer.Write(resourceWithUID("uid-1", "100"), broker.Add, cfg))
	assert.NoError(t, writer.Write(resourceWithUID("uid-1", "101"), broker.Update, cfg))
	assert.NoError(t, writer.Write(resourceWithUID("uid-1", "102"), broker.Update, cfg))

	assert.Len(t, mock.written, 3)

	// A repeat of the last content is then suppressed, proving the stored hash
	// tracks the most recent payload.
	assert.NoError(t, writer.Write(resourceWithUID("uid-1", "102"), broker.Update, cfg))
	assert.Len(t, mock.written, 3)
}

func TestContentDeduplicatorWriter_AlwaysEmitsDeleteAndEvicts(t *testing.T) {
	mock := &mockWriter{}
	writer := NewContentDeduplicatorWriter(mock)
	cfg := config.PipelineConfig{}

	res := resourceWithUID("uid-1", "100")

	assert.NoError(t, writer.Write(res, broker.Add, cfg))    // published
	assert.NoError(t, writer.Write(res, broker.Delete, cfg)) // always published
	// A DELETE with content identical to a prior publish must still go out.
	assert.NoError(t, writer.Write(res, broker.Delete, cfg))

	assert.Len(t, mock.written, 3)
	assert.Equal(t, broker.Add, mock.events[0])
	assert.Equal(t, broker.Delete, mock.events[1])
	assert.Equal(t, broker.Delete, mock.events[2])

	// Eviction check: after DELETE, re-adding the same UID with the SAME content
	// as the original ADD must republish (the hash was evicted, not retained).
	mock2 := &mockWriter{}
	writer2 := NewContentDeduplicatorWriter(mock2)
	assert.NoError(t, writer2.Write(res, broker.Add, cfg))
	assert.NoError(t, writer2.Write(res, broker.Delete, cfg))
	assert.NoError(t, writer2.Write(res, broker.Add, cfg)) // re-created UID, fresh publish
	assert.Len(t, mock2.written, 3)
	assert.Equal(t, broker.Add, mock2.events[2])

	// The map must not retain the UID after delete.
	writer2.mu.Lock()
	_, present := writer2.hashByUID["uid-1"]
	writer2.mu.Unlock()
	assert.True(t, present, "re-add after delete should have re-populated the UID")
}

func TestContentDeduplicatorWriter_MapBoundedByDelete(t *testing.T) {
	mock := &mockWriter{}
	writer := NewContentDeduplicatorWriter(mock)
	cfg := config.PipelineConfig{}

	res := resourceWithUID("uid-1", "100")
	assert.NoError(t, writer.Write(res, broker.Add, cfg))

	writer.mu.Lock()
	_, present := writer.hashByUID["uid-1"]
	writer.mu.Unlock()
	assert.True(t, present)

	assert.NoError(t, writer.Write(res, broker.Delete, cfg))

	writer.mu.Lock()
	_, present = writer.hashByUID["uid-1"]
	size := len(writer.hashByUID)
	writer.mu.Unlock()
	assert.False(t, present, "DELETE must evict the UID from the map")
	assert.Equal(t, 0, size)
}

func TestContentDeduplicatorWriter_NeverDedupsEmptyUID(t *testing.T) {
	mock := &mockWriter{}
	writer := NewContentDeduplicatorWriter(mock)
	cfg := config.PipelineConfig{}

	// Nil metadata and empty-string UID are both treated as "no UID" and must
	// always be forwarded, even for byte-identical repeats.
	noMeta := model.KubernetesResource{KubernetesResourceMeta: nil}
	emptyUID := model.KubernetesResource{
		KubernetesResourceMeta: &model.KubernetesResourceObjectMeta{UID: ""},
	}

	assert.NoError(t, writer.Write(noMeta, broker.Add, cfg))
	assert.NoError(t, writer.Write(noMeta, broker.Add, cfg))
	assert.NoError(t, writer.Write(emptyUID, broker.Update, cfg))
	assert.NoError(t, writer.Write(emptyUID, broker.Update, cfg))

	assert.Len(t, mock.written, 4)

	// Empty-UID resources must never populate the dedup map.
	writer.mu.Lock()
	size := len(writer.hashByUID)
	writer.mu.Unlock()
	assert.Equal(t, 0, size)
}

func TestContentDeduplicatorWriter_DistinctUIDsIndependent(t *testing.T) {
	mock := &mockWriter{}
	writer := NewContentDeduplicatorWriter(mock)
	cfg := config.PipelineConfig{}

	// Identical content on two different UIDs must both publish: dedup is
	// per-UID, not global content dedup.
	a := resourceWithUID("uid-a", "100")
	b := resourceWithUID("uid-b", "100")

	assert.NoError(t, writer.Write(a, broker.Add, cfg))
	assert.NoError(t, writer.Write(b, broker.Add, cfg))
	// Repeats of each are then suppressed.
	assert.NoError(t, writer.Write(a, broker.Update, cfg))
	assert.NoError(t, writer.Write(b, broker.Update, cfg))

	assert.Len(t, mock.written, 2)
}

// TestContentDeduplicatorWriter_ConcurrentWrites exercises the mutex under the
// race detector. Each goroutine owns a distinct UID and writes the same content
// repeatedly; exactly one publish per UID must survive dedup.
func TestContentDeduplicatorWriter_ConcurrentWrites(t *testing.T) {
	mock := &lockingMockWriter{}
	writer := NewContentDeduplicatorWriter(mock)
	cfg := config.PipelineConfig{}

	const goroutines = 16
	const perGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			res := resourceWithUID(fmt.Sprintf("uid-%d", id), "100")
			for i := 0; i < perGoroutine; i++ {
				assert.NoError(t, writer.Write(res, broker.Update, cfg))
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, goroutines, mock.count())
}

// lockingMockWriter is a minimal thread-safe Writer for the concurrency test;
// the package-shared mockWriter is not safe for concurrent use.
type lockingMockWriter struct {
	mu      sync.Mutex
	written int
}

func (m *lockingMockWriter) Write(_ model.KubernetesResource, _ broker.EventType, _ config.PipelineConfig) error {
	m.mu.Lock()
	m.written++
	m.mu.Unlock()
	return nil
}

func (m *lockingMockWriter) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.written
}
