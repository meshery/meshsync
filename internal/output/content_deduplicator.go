package output

import (
	"crypto/sha256"
	"encoding/json"
	"sync"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
)

// ContentDeduplicatorWriter is a streaming output wrapper that suppresses
// byte-identical republishes of the SAME resource on the broker path.
//
// It keys resources by KubernetesResourceMeta.UID and remembers a sha256 hash
// of the last payload published for that UID. On ADD/UPDATE, if the incoming
// payload hashes to the same value already recorded for the UID, the write is
// skipped; otherwise it is forwarded and the recorded hash is updated. This is
// a bandwidth/DB-write optimisation on top of the resourceVersion-based
// suppression already performed in the informer UpdateFunc: two distinct
// resourceVersions can still carry identical wire content (e.g. status churn
// that normalises away), and only a content hash catches those.
//
// The full object is always forwarded when it IS published: this wrapper never
// rewrites the wire format into a delta/patch, because Meshery Server consumes
// full objects from these subjects.
//
// Invariants that keep this safe:
//   - DELETE is ALWAYS forwarded and evicts the UID from the map. Eviction
//     bounds the map to the set of currently-live UIDs and guarantees that a
//     re-created object (same UID reused, or a new UID) republishes fresh.
//   - Resources with an empty/absent UID are never deduplicated; each is
//     forwarded as-is (mirrors InMemoryDeduplicatorStreamingWriter).
//   - Ordering and event semantics are preserved: this is a pass-through filter,
//     it never reorders, batches, or defers events.
//
// NOTE on informer resyncs: the broker writer (and therefore this wrapper)
// persists across informer resyncs - a resync recreates the informer factory
// but not the output writer - so the hash map survives a resync. A resync
// re-lists every object, and unchanged objects would be suppressed here. That
// is acceptable while Meshery Server still holds those unchanged objects, but
// because a resync is also a recovery path it must be an explicit opt-in
// (see config.EnvBrokerContentDedup); it is OFF by default so the default
// behaviour - republish everything - is unchanged.
type ContentDeduplicatorWriter struct {
	realWriter Writer

	mu sync.Mutex
	// hashByUID stores the sha256 of the last payload published per resource UID.
	// Entries are added/updated on ADD/UPDATE and removed on DELETE, so the map
	// stays bounded to the number of live resources with a UID.
	hashByUID map[string][sha256.Size]byte
}

// NewContentDeduplicatorWriter wraps realWriter with content-hash
// deduplication keyed by resource UID.
func NewContentDeduplicatorWriter(realWriter Writer) *ContentDeduplicatorWriter {
	return &ContentDeduplicatorWriter{
		realWriter: realWriter,
		hashByUID:  make(map[string][sha256.Size]byte),
	}
}

func (w *ContentDeduplicatorWriter) Write(
	obj model.KubernetesResource,
	evtype broker.EventType,
	cfg config.PipelineConfig,
) error {
	uid := ""
	if obj.KubernetesResourceMeta != nil {
		uid = obj.KubernetesResourceMeta.UID
	}

	// No UID: cannot be tracked reliably, so always forward (never dedup).
	if uid == "" {
		return w.realWriter.Write(obj, evtype, cfg)
	}

	// DELETE always publishes and evicts the UID. Evicting keeps the map bounded
	// and ensures a subsequently re-created object republishes fresh rather than
	// colliding with a stale hash.
	if evtype == broker.Delete {
		w.mu.Lock()
		delete(w.hashByUID, uid)
		w.mu.Unlock()
		return w.realWriter.Write(obj, evtype, cfg)
	}

	// ADD/UPDATE: publish only when the payload content changed for this UID.
	// Hash outside the lock: the JSON serialization is CPU-bound and must not
	// serialize concurrent writers. The mutex guards only the map, and the
	// downstream write happens after the lock is released.
	hash, err := hashResource(obj)
	if err != nil {
		// Hashing failed for reasons outside our control (a value that does not
		// round-trip through JSON). Fail open - forward the event - rather than
		// silently dropping it, and drop any stale hash so we do not wedge this
		// UID into a permanently-suppressed state.
		w.mu.Lock()
		delete(w.hashByUID, uid)
		w.mu.Unlock()
		return w.realWriter.Write(obj, evtype, cfg)
	}

	w.mu.Lock()
	prev, ok := w.hashByUID[uid]
	w.mu.Unlock()
	if ok && prev == hash {
		// Byte-identical to the last successfully published payload: skip.
		return nil
	}

	// Publish first, then record the hash only on success. Recording before the
	// write would let a failed publish suppress a later retry of the same payload
	// (the payload would be marked "published" though it never was).
	if err := w.realWriter.Write(obj, evtype, cfg); err != nil {
		return err
	}

	w.mu.Lock()
	w.hashByUID[uid] = hash
	w.mu.Unlock()
	return nil
}

// hashResource returns the sha256 of the JSON encoding of obj. json.Marshal is
// deterministic for a given value (struct fields in declaration order, map keys
// sorted), and the object is the same value that meshkit marshals onto the wire,
// so identical published payloads hash identically.
func hashResource(obj model.KubernetesResource) ([sha256.Size]byte, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(data), nil
}
