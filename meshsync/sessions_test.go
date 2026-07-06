package meshsync

import (
	"fmt"
	"sync"
	"testing"

	"github.com/meshery/meshsync/internal/channels"
)

func newSessionsHandler() *Handler {
	return &Handler{sessions: make(map[string]channels.StructChannel)}
}

func TestAddSessionIsIdempotent(t *testing.T) {
	h := newSessionsHandler()

	ch1, created1 := h.addSession("a")
	if !created1 {
		t.Fatal("first addSession should report created=true")
	}
	ch2, created2 := h.addSession("a")
	if created2 {
		t.Fatal("second addSession for the same id should report created=false")
	}
	if ch1 != ch2 {
		t.Fatal("addSession should return the same channel for an existing id")
	}
	if _, ok := h.getSession("a"); !ok {
		t.Fatal("getSession should find the added session")
	}
	if got := h.activeSessionIDs(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("activeSessionIDs = %v, want [a]", got)
	}

	h.deleteSession("a")
	if _, ok := h.getSession("a"); ok {
		t.Fatal("getSession should not find a deleted session")
	}
	if got := h.activeSessionIDs(); len(got) != 0 {
		t.Fatalf("activeSessionIDs after delete = %v, want []", got)
	}
	// deleteSession on a missing id must be a no-op, not a panic.
	h.deleteSession("missing")
}

// TestSessionsConcurrentAccess must pass under -race: many goroutines add, read,
// enumerate, and delete overlapping session ids simultaneously. Before sessions
// were split out of channelPool, this shape of access was a concurrent map
// read/write.
func TestSessionsConcurrentAccess(t *testing.T) {
	h := newSessionsHandler()

	const workers = 16
	const iterations = 500
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			id := fmt.Sprintf("session-%d", w%4) // deliberate overlap across workers
			for i := 0; i < iterations; i++ {
				h.addSession(id)
				h.getSession(id)
				h.activeSessionIDs()
				h.deleteSession(id)
			}
		}(w)
	}
	wg.Wait()
}

func TestAddSessionChannelIsBuffered(t *testing.T) {
	h := newSessionsHandler()
	ch, created := h.addSession("s")
	if !created {
		t.Fatal("expected the session to be created")
	}
	// The session channel must be buffered: exec/log-stream send stop signals
	// non-blocking, so a stop that arrives before the receiver is ready must land
	// in the buffer rather than being dropped.
	select {
	case ch <- struct{}{}:
	default:
		t.Fatal("session channel is unbuffered: a non-blocking stop signal was dropped")
	}
}

func TestAddSessionInitializesNilMap(t *testing.T) {
	// A Handler constructed outside New has a nil sessions map; addSession must
	// initialize it rather than panic on the write.
	h := &Handler{}
	ch, created := h.addSession("s")
	if !created || ch == nil {
		t.Fatal("addSession should lazily initialize the map and create the session")
	}
	if _, ok := h.getSession("s"); !ok {
		t.Fatal("session should be retrievable after lazy initialization")
	}
}
