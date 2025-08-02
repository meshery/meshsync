package output

import (
	"sync"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
)

// InMemoryDeduplicatorStreamingWriter writes each unique resource once immediately upon first seeing it
type InMemoryDeduplicatorStreamingWriter struct {
	realWriter Writer

	// deduplication storage
	mu             sync.Mutex
	seenUIDs       map[string]struct{}
	seenNoMetaUIDs []model.KubernetesResource // for resources with no UID (assumed unique)
}

// NewImmediateDeduplicatorWriter creates a deduplicator that writes immediately and filters repeats
func NewInMemoryDeduplicatorStreamingWriter(realWriter Writer) *InMemoryDeduplicatorStreamingWriter {
	return &InMemoryDeduplicatorStreamingWriter{
		realWriter:     realWriter,
		seenUIDs:       make(map[string]struct{}),
		seenNoMetaUIDs: make([]model.KubernetesResource, 0, 128),
	}
}

func (w *InMemoryDeduplicatorStreamingWriter) Write(
	obj model.KubernetesResource,
	evtype broker.EventType,
	cfg config.PipelineConfig,
) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Case 1: UID is present — deduplicate by UID
	if obj.KubernetesResourceMeta != nil && obj.KubernetesResourceMeta.UID != "" {
		uid := obj.KubernetesResourceMeta.UID
		if _, seen := w.seenUIDs[uid]; seen {
			return nil // duplicate, skip
		}
		// mark as seen and write
		w.seenUIDs[uid] = struct{}{}
		return w.realWriter.Write(obj, evtype, cfg)
	}

	// Case 2: No UID — treat as unique and write
	return w.realWriter.Write(obj, evtype, cfg)
}
