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
		if _, ok := h.channelPool[id]; !ok {
			// Subscribing the first time
			if !bool(req.Stop) {
				h.channelPool[id] = channels.NewStructChannel()
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
		} else {
			// Already running subscription
			if bool(req.Stop) {
				// TODO: once we have a unsubscribe functionality, need to publish message to active sessions subject
				execCleanup(h, id)
			}
		}
	}

	return nil
}
func (h *Handler) processActiveExecRequest() error {
	go h.streamChannelPool()

	return nil
}
func (h *Handler) getActiveChannels() []*string {
	activeChannels := make([]*string, 0, len(h.channelPool))
	for k := range h.channelPool {
		activeChannels = append(activeChannels, &k)
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

// TODO fix cyclop error
// Error: meshsync/exec.go:113:1: calculated cyclomatic complexity for function streamSession is 15, max is 10 (cyclop)
//
//nolint:cyclop
func (h *Handler) streamSession(id string, req model.ExecRequest, cfg config.ListenerConfig) {
	subCh := make(chan *broker.Message)
	tstdin, putStdin := io.Pipe()
	stdin := io.NopCloser(tstdin)
	getStdout, stdout := io.Pipe()

	// done is closed exactly once when the session ends (stream EOF/error,
	// explicit Stop, or the global channels.Stop). Closing the pipes unblocks the
	// stdout reader and any in-flight stdin write, so no goroutine is left blocked.
	done := make(chan struct{})
	var once sync.Once
	terminate := func() {
		once.Do(func() {
			close(done)
			// Closing both ends of both pipes unblocks the TTY streamer, the
			// stdout reader (Read returns io.ErrClosedPipe) and any stdin writer.
			_ = putStdin.Close()
			_ = tstdin.Close()
			_ = stdout.Close()
			_ = getStdout.Close()
			delete(h.channelPool, id)
		})
	}
	defer terminate()

	if err := h.Broker.SubscribeWithChannel(fmt.Sprintf("input.%s", id), generateID(), subCh); err != nil {
		h.Log.Error(ErrExecTerminal(err))
	}

	// The broker interface exposes no Unsubscribe, so the input.<id> subscription
	// cannot be torn down here. Once the session ends, keep draining subCh so the
	// broker's delivery goroutine never blocks on an unread channel after we stop.
	go func() {
		<-done
		for range subCh {
		}
	}()

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
		for {
			data := make([]byte, 1*KB)
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
		// The session's StructChannel is asserted below; once terminate() has
		// removed the pool entry that assertion would be on a nil interface and
		// panic, so bail out first if the session is already gone.
		sessionCh, ok := h.channelPool[id].(channels.StructChannel)
		if !ok {
			h.Log.Debugf("Session closed for: %s", id)
			return
		}

		select {
		case msg := <-subCh:
			if msg.ObjectType == broker.ExecInputObject {
				if _, err := io.CopyBuffer(putStdin, strings.NewReader(msg.Object.(string)+"\n"), nil); err != nil {
					h.Log.Error(ErrExecTerminal(err))
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

func execCleanup(h *Handler, id string) {
	ch, ok := h.channelPool[id]
	if !ok {
		return
	}

	structChan, ok := ch.(channels.StructChannel)
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
