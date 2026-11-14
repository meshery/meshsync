package meshsync

import (
	"fmt"
	"sync"
	"testing"

	"k8s.io/client-go/tools/cache"
)

func newStoresHandler() *Handler {
	return &Handler{stores: make(map[string]cache.Store)}
}

// TestStoresConcurrentAccess must pass under -race: the discovery goroutine
// replaces the stores map wholesale on every (re)discovery while the request
// listener goroutine snapshots it to answer informer-store requests. Without
// storesMu that is an unsynchronized read/write of the map field.
func TestStoresConcurrentAccess(t *testing.T) {
	h := newStoresHandler()

	const workers = 16
	const iterations = 500
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if w%2 == 0 {
					h.replaceStores(map[string]cache.Store{
						fmt.Sprintf("gvr-%d", w): cache.NewStore(cache.MetaNamespaceKeyFunc),
					})
				} else {
					for _, s := range h.snapshotStores() {
						_ = s.List()
					}
				}
			}
		}(w)
	}
	wg.Wait()
}

// TestSnapshotStoresReturnsAllStores verifies snapshotStores returns every
// current store so informer-store replies stay complete after the guard.
func TestSnapshotStoresReturnsAllStores(t *testing.T) {
	h := newStoresHandler()
	h.replaceStores(map[string]cache.Store{
		"a": cache.NewStore(cache.MetaNamespaceKeyFunc),
		"b": cache.NewStore(cache.MetaNamespaceKeyFunc),
		"c": cache.NewStore(cache.MetaNamespaceKeyFunc),
	})
	if got := len(h.snapshotStores()); got != 3 {
		t.Fatalf("snapshotStores returned %d stores, want 3", got)
	}
}

// TestSnapshotStoresNilMap confirms a Handler whose stores map was never
// initialized (nil, as before the first discovery) snapshots to empty rather
// than panicking.
func TestSnapshotStoresNilMap(t *testing.T) {
	h := &Handler{}
	if got := len(h.snapshotStores()); got != 0 {
		t.Fatalf("snapshotStores on a nil map returned %d, want 0", got)
	}
}
