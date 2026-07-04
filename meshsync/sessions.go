package meshsync

import (
	"github.com/meshery/meshsync/internal/channels"
)

// Interactive exec and log-stream sessions are keyed by a per-request id.
// They were previously stored in the shared channelPool alongside the fixed
// system channels (Stop/OS/ReSync), which meant session goroutines mutated the
// same map that other goroutines read (system-channel selects, getActiveChannels),
// a data race that can panic the process. They now live in their own
// mutex-guarded map so channelPool stays read-only after initialization.
//
// Every helper returns the channel (if any) and releases the lock before the
// caller performs any channel send/receive, so the sessions mutex is never held
// across a blocking channel operation.

// addSession registers a new session channel for id and returns it with
// created=true. If a session already exists for id, the existing channel is
// returned with created=false so the caller does not start a duplicate.
func (h *Handler) addSession(id string) (ch channels.StructChannel, created bool) {
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()
	if existing, ok := h.sessions[id]; ok {
		return existing, false
	}
	ch = channels.NewStructChannel()
	h.sessions[id] = ch
	return ch, true
}

// getSession returns the session channel for id, if present.
func (h *Handler) getSession(id string) (channels.StructChannel, bool) {
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()
	ch, ok := h.sessions[id]
	return ch, ok
}

// deleteSession removes the session for id. It is safe to call for an id that
// is not present.
func (h *Handler) deleteSession(id string) {
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()
	delete(h.sessions, id)
}

// activeSessionIDs returns the ids of the currently active sessions.
func (h *Handler) activeSessionIDs() []string {
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()
	ids := make([]string, 0, len(h.sessions))
	for id := range h.sessions {
		ids = append(ids, id)
	}
	return ids
}
