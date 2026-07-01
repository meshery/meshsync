package meshsync

import (
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/meshery/meshkit/logger"
)

// healthServer exposes minimal liveness and readiness endpoints:
//
//   - GET /healthz always returns 200 and only signals that the process
//     is alive.
//   - GET /readyz returns 200 once markReady has been called (i.e. the
//     broker handler has been created successfully) and 503 before that.
type healthServer struct {
	ready atomic.Bool
}

func newHealthServer() *healthServer {
	return &healthServer{}
}

// markReady flips the readiness flag so that /readyz starts returning 200.
func (h *healthServer) markReady() {
	h.ready.Store(true)
}

// handler returns the http.Handler serving the health endpoints.
func (h *healthServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !h.ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

// start serves the health endpoints on addr in a background goroutine and
// returns a stop function that shuts the server down (releasing the port and
// goroutine — Run is a library entry point, so callers may outlive it).
// A failure to serve (e.g. the port is already taken) is logged and otherwise
// ignored: health reporting must never take the process down.
func (h *healthServer) start(log logger.Handler, addr string) func() {
	srv := &http.Server{
		Addr:              addr,
		Handler:           h.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Infof("meshsync: health endpoints /healthz and /readyz listening on %s", addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warnf("meshsync: health endpoints server on %s stopped: %v", addr, err)
		}
	}()
	return func() {
		if err := srv.Close(); err != nil {
			log.Warnf("meshsync: health endpoints server close failed: %v", err)
		}
	}
}
