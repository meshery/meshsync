// Copyright Meshery Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package meshsync

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshsync/internal/channels"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/util/interrupt"
	"k8s.io/kubectl/pkg/util/term"
)

// KB stands for KiloByte
const KB = 1024

// execInputChannelBuffer bounds how many stdin messages can queue for a session
// before the broker's delivery goroutine backpressures. A cushion (rather than
// an unbuffered channel) keeps a burst of input - or a delivery already in
// flight when the session tears down - from blocking that delivery goroutine in
// the window before Unsubscribe stops further delivery. It is a cushion, not a
// correctness dependency: teardown unsubscribes the subject regardless.
const execInputChannelBuffer = 256

// terminalSizeQueueAdapter adapts kubectl's term.TerminalSizeQueue to client-go's remotecommand.TerminalSizeQueue
type terminalSizeQueueAdapter struct {
	queue term.TerminalSizeQueue
}

// Next implements remotecommand.TerminalSizeQueue by converting term.TerminalSize to remotecommand.TerminalSize
func (a *terminalSizeQueueAdapter) Next() *remotecommand.TerminalSize {
	termSize := a.queue.Next()
	if termSize == nil {
		return nil
	}
	return &remotecommand.TerminalSize{
		Width:  termSize.Width,
		Height: termSize.Height,
	}
}

func (h *Handler) processExecRequest(obj interface{}, cfg config.ListenerConfig) error {
	reqs := make(model.ExecRequests)
	d, err := utils.Marshal(obj)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(d), &reqs)
	if err != nil {
		return err
	}

	for _, req := range reqs {
		id := fmt.Sprintf("exec.%s.%s.%s.%s", req.Namespace, req.Name, req.Container, req.ID)
		if bool(req.Stop) {
			// Stop request: tear down the running session if any (no-op otherwise).
			// The session's terminate() unsubscribes its input.<id> subject; the
			// 10s streamChannelPool ticker republishes the active-session list.
			execCleanup(h, id)
			continue
		}
		if _, created := h.addSession(id); !created {
			// A session for this id is already running.
			continue
		}
		h.Log.Debug("Starting session")

		err := h.Broker.Publish("active_sessions.exec", &broker.Message{
			ObjectType: broker.ActiveExecObject,
			Object:     h.getActiveChannels(),
		})
		if err != nil {
			h.Log.Error(ErrGetObject(err))
		}
		go h.streamSession(id, req, cfg)
	}

	return nil
}
func (h *Handler) processActiveExecRequest() error {
	go h.streamChannelPool()

	return nil
}
func (h *Handler) getActiveChannels() []*string {
	ids := h.activeSessionIDs()
	activeChannels := make([]*string, 0, len(ids))
	for i := range ids {
		activeChannels = append(activeChannels, &ids[i])
	}

	return activeChannels
}

func (h *Handler) streamChannelPool() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		publish := func() {
			err := h.Broker.Publish("active_sessions.exec", &broker.Message{
				ObjectType: broker.ActiveExecObject,
				Object:     h.getActiveChannels(),
			})
			if err != nil {
				h.Log.Error(ErrGetObject(err))
			}
		}

	loop:
		for {
			select {
			case <-h.channelPool[channels.Stop].(channels.StopChannel):
				break loop
			case <-ticker.C:
				publish()
			}
		}
		h.Log.Debug("Stopping streamChannelPool")
	}()
}

// streamSession wires up one exec session: its input subscription and teardown,
// the TTY exec stream, stdout publishing, and the input loop. Its branch count
// (the four-case select plus the teardown/streaming closures) exceeds cyclop's
// default; the body is a linear setup-then-loop sequence, so splitting it would
// scatter the shared pipes/channels and hurt readability more than it helps.
//
//nolint:cyclop
func (h *Handler) streamSession(id string, req model.ExecRequest, cfg config.ListenerConfig) {
	// Buffered (see execInputChannelBuffer) so a burst of stdin - or a delivery
	// already in flight at teardown - lands in the buffer instead of blocking the
	// broker's delivery goroutine on an unread channel in the window before
	// Unsubscribe (in terminate) takes effect.
	subCh := make(chan *broker.Message, execInputChannelBuffer)
	tstdin, putStdin := io.Pipe()
	stdin := io.NopCloser(tstdin)
	getStdout, stdout := io.Pipe()

	// inputSubject is the per-session subject the client publishes stdin to; it is
	// subscribed below and torn down in terminate().
	inputSubject := fmt.Sprintf("input.%s", id)

	// done is closed exactly once when the session ends (stream EOF/error,
	// explicit Stop, or the global channels.Stop). Closing the pipes unblocks the
	// stdout reader and any in-flight stdin write, so no goroutine is left blocked.
	done := make(chan struct{})
	var once sync.Once
	terminate := func() {
		once.Do(func() {
			close(done)
			// Tear down the input subscription so the broker stops delivering to
			// subCh and releases the delivery goroutine it started for it. The main
			// loop has already stopped reading subCh (it returns on done), so
			// without this the subscription and its goroutine would leak for the
			// process lifetime. Unsubscribe is idempotent, so the repeated
			// terminate() calls (defer + TTY goroutine) are safe.
			h.unsubscribeSessionInput(inputSubject)
			// Closing both ends of both pipes unblocks the TTY streamer, the
			// stdout reader (Read returns io.ErrClosedPipe) and any stdin writer.
			_ = putStdin.Close()
			_ = tstdin.Close()
			_ = stdout.Close()
			_ = getStdout.Close()
			h.deleteSession(id)
		})
	}
	defer terminate()

	if err := h.Broker.SubscribeWithChannel(inputSubject, generateID(), subCh); err != nil {
		h.Log.Error(ErrExecTerminal(err))
	}

	// Put the terminal into raw mode to prevent it echoing characters twice.
	t := term.TTY{
		Parent: interrupt.New(func(s os.Signal) {}),
		Out:    stdout,
		In:     stdin,
		Raw:    true,
	}
	sizeQueue := &terminalSizeQueueAdapter{queue: t.MonitorSize(t.GetSize())}

	// TTY request GoRoutine
	go func() {
		defer terminate()

		fn := func() error {
			request := h.kubeClient.KubeClient.CoreV1().RESTClient().Post().
				Namespace(req.Namespace).
				Resource("pods").
				Name(req.Name).
				SubResource("exec")
			request.VersionedParams(&corev1.PodExecOptions{
				Container: req.Container,
				Command:   []string{"/bin/sh"},
				Stdin:     true,
				Stdout:    true,
				Stderr:    true,
				TTY:       true,
			}, scheme.ParameterCodec)

			exec, postErr := remotecommand.NewSPDYExecutor(&h.kubeClient.RestConfig, "POST", request.URL())
			if postErr != nil {
				return postErr
			}

			return exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
				Stdin:             stdin,
				Stdout:            stdout,
				Stderr:            stdout,
				Tty:               true,
				TerminalSizeQueue: sizeQueue})
		}

		if err := t.Safe(fn); err != nil {
			h.Log.Error(ErrExecTerminal(err))

			// If the TTY fails then send the error message to the client
			if pubErr := h.Broker.Publish(id, &broker.Message{
				ObjectType: broker.ErrorObject,
				Object:     err.Error(),
			}); pubErr != nil {
				h.Log.Error(ErrExecTerminal(pubErr))
			}
		}
	}()

	// TTY stdout streaming Goroutine
	go func() {
		rdr := bufio.NewReader(getStdout)
		// Reused across iterations: string(data[:n]) copies the bytes on publish,
		// so a single buffer is safe and avoids a 1KB allocation per read.
		data := make([]byte, 1*KB)
		for {
			n, err := rdr.Read(data)
			if n > 0 {
				// Publish only the bytes actually read to avoid emitting the
				// unused trailing NUL padding of the buffer.
				if pubErr := h.Broker.Publish(id, &broker.Message{
					ObjectType: broker.ExecOutputObject,
					Object:     string(data[:n]),
				}); pubErr != nil {
					h.Log.Error(ErrExecTerminal(pubErr))
				}
			}
			if err != nil {
				// EOF or a closed pipe (session terminated) ends the reader. No
				// cleanup on EOF alone as it can be a false positive mid-session.
				return
			}
		}
	}()

	for {
		// If terminate() has already removed the session, bail out.
		sessionCh, ok := h.getSession(id)
		if !ok {
			h.Log.Debugf("Session closed for: %s", id)
			return
		}

		select {
		case msg, ok := <-subCh:
			if !ok {
				// A broker implementation that closes the delivery channel on
				// Unsubscribe would make this receive return (nil, false); end the
				// session rather than spin on the closed channel or dereference a
				// nil message.
				h.Log.Debugf("Input channel closed for session %s", id)
				return
			}
			// Guard the payload assertion too: a malformed ExecInput message with a
			// non-string object must not panic the session loop.
			if msg != nil && msg.ObjectType == broker.ExecInputObject {
				if input, isStr := msg.Object.(string); isStr {
					if _, err := io.CopyBuffer(putStdin, strings.NewReader(input+"\n"), nil); err != nil {
						h.Log.Error(ErrExecTerminal(err))
					}
				}
			}
		case <-sessionCh:
			h.Log.Debugf("Closing session %s", id)
			return
		case <-h.channelPool[channels.Stop].(channels.StopChannel):
			h.Log.Debugf("Stopping session %s on global stop", id)
			return
		case <-done:
			h.Log.Debugf("Session closed for: %s", id)
			return
		}
	}
}

// unsubscribeSessionInput tears down an exec session's stdin subscription
// (input.<id>) so the broker stops delivering to its channel and releases the
// delivery goroutine it started for it. It runs during session teardown, so the
// error is logged rather than returned; Unsubscribe is a no-op for a subject
// with no active subscription and is safe to call more than once.
func (h *Handler) unsubscribeSessionInput(subject string) {
	if err := h.Broker.Unsubscribe(subject); err != nil {
		h.Log.Error(ErrExecTerminal(err))
	}
}

func execCleanup(h *Handler, id string) {
	structChan, ok := h.getSession(id)
	if !ok {
		return
	}

	// Non-blocking: the session's loop terminates and stops receiving on this
	// channel from several paths, so a plain send could deadlock the caller once
	// the session has already ended. Dropping the signal is safe because the
	// session tears itself down independently.
	select {
	case structChan <- struct{}{}:
	default:
	}
}

func generateID() string {
	return uuid.New().String()
}
