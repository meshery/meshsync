package meshsync

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/broker/channel"
	"github.com/meshery/meshkit/logger"
)

// recordingBroker wraps a real broker.Handler, recording the subjects passed to
// Unsubscribe (and optionally forcing it to fail) while delegating every other
// method - including the actual teardown - to the embedded handler.
type recordingBroker struct {
	broker.Handler
	mu           sync.Mutex
	unsubscribed []string
	failWith     error
}

// Compile-time proof that the fake satisfies the (extended) broker.Handler
// interface; if a new method is added to the interface this fails to build.
var _ broker.Handler = (*recordingBroker)(nil)

func (r *recordingBroker) Unsubscribe(subject string) error {
	r.mu.Lock()
	r.unsubscribed = append(r.unsubscribed, subject)
	r.mu.Unlock()
	if r.failWith != nil {
		return r.failWith
	}
	return r.Handler.Unsubscribe(subject)
}

func (r *recordingBroker) subjects() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.unsubscribed...)
}

func newTestLogger(t *testing.T) logger.Handler {
	t.Helper()
	log, err := logger.New("meshsync-test", logger.Options{})
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	return log
}

// TestUnsubscribeSessionInputTearsDownSubscription is the regression test for
// the exec input-subscription/goroutine leak (meshery/meshsync#585): before
// broker.Handler exposed Unsubscribe, streamSession could not tear down its
// input.<id> subscription and parked a drain goroutine that never exited,
// leaking a subscription and a goroutine per exec session. Teardown must now
// unsubscribe the per-session subject, which releases the broker's delivery
// goroutine.
func TestUnsubscribeSessionInputTearsDownSubscription(t *testing.T) {
	rec := &recordingBroker{Handler: channel.NewChannelBrokerHandler()}
	h := &Handler{Broker: rec, Log: newTestLogger(t)}

	const id = "exec.ns.pod.ctr.req-1"
	subject := "input." + id
	// Mirror streamSession: a 1-buffered channel subscribed to input.<id>.
	subCh := make(chan *broker.Message, 1)

	baseline := runtime.NumGoroutine()

	if err := rec.SubscribeWithChannel(subject, generateID(), subCh); err != nil {
		t.Fatalf("SubscribeWithChannel: %v", err)
	}

	// Sanity: a published input message reaches the session channel while the
	// subscription is live.
	if err := rec.Publish(subject, &broker.Message{ObjectType: broker.ExecInputObject, Object: "hi"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	select {
	case msg := <-subCh:
		if got, _ := msg.Object.(string); got != "hi" {
			t.Fatalf("delivered %q, want %q", got, "hi")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("input message was not delivered before Unsubscribe")
	}

	// Behavior under test: session teardown unsubscribes the per-session subject.
	h.unsubscribeSessionInput(subject)

	if got := rec.subjects(); len(got) != 1 || got[0] != subject {
		t.Fatalf("Unsubscribe called with %v, want [%s]", got, subject)
	}

	// The real broker actually tore the subscription down: nothing is left
	// registered on the subject.
	for _, ep := range rec.ConnectedEndpoints() {
		if strings.HasPrefix(ep, subject+"::") {
			t.Fatalf("subject %s still registered after Unsubscribe: %v", subject, rec.ConnectedEndpoints())
		}
	}

	// ...and the delivery goroutine it started has exited, so nothing leaks.
	waitForGoroutines(t, baseline, 3*time.Second)
}

// TestUnsubscribeSessionInputLogsErrorWithoutPanic covers the teardown error
// path: unsubscribeSessionInput runs during session cleanup, so an Unsubscribe
// error must be logged rather than propagated or panicked on.
func TestUnsubscribeSessionInputLogsErrorWithoutPanic(t *testing.T) {
	rec := &recordingBroker{
		Handler:  channel.NewChannelBrokerHandler(),
		failWith: context.Canceled, // stand-in broker failure
	}
	h := &Handler{Broker: rec, Log: newTestLogger(t)}

	// Must not panic even though Unsubscribe fails.
	h.unsubscribeSessionInput("input.exec.ns.pod.ctr.req-err")

	if got := rec.subjects(); len(got) != 1 {
		t.Fatalf("expected exactly one Unsubscribe call, got %v", got)
	}
}

// waitForGoroutines fails if the goroutine count has not returned to at most
// baseline within timeout. It guards against the per-session delivery-goroutine
// leak that the old drain-goroutine approach caused.
func waitForGoroutines(t *testing.T, baseline int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		runtime.Gosched()
		if runtime.NumGoroutine() <= baseline {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("goroutine count did not return to baseline %d (now %d): delivery goroutine leaked",
				baseline, runtime.NumGoroutine())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
