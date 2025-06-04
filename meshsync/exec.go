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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshsync/internal/channels"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/util/interrupt"
	"k8s.io/kubectl/pkg/util/term"
)

// KB stands for KiloByte
const KB = 1024

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
				h.Log.Info("Starting session")

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
		for {
			err := h.Broker.Publish("active_sessions.exec", &broker.Message{
				ObjectType: broker.ActiveExecObject,
				Object:     h.getActiveChannels(),
			})
			if err != nil {
				h.Log.Error(ErrGetObject(err))
			}

			time.Sleep(10 * time.Second)
		}
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

	err := h.Broker.SubscribeWithChannel(fmt.Sprintf("input.%s", id), generateID(), subCh)
	if err != nil {
		h.Log.Error(ErrExecTerminal(err))
	}

	// Put the terminal into raw mode to prevent it echoing characters twice.
	t := term.TTY{
		Parent: interrupt.New(func(s os.Signal) {}),
		Out:    stdout,
		In:     stdin,
		Raw:    true,
	}
	sizeQueue := t.MonitorSize(t.GetSize())

	// TTY request GoRoutine
	go func() {
		fn := func() error {
			request := h.staticClient.CoreV1().RESTClient().Post().
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

			exec, postErr := remotecommand.NewSPDYExecutor(&h.restConfig, "POST", request.URL())
			if postErr != nil {
				return err
			}

			err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
				Stdin:             stdin,
				Stdout:            stdout,
				Stderr:            stdout,
				Tty:               true,
				TerminalSizeQueue: sizeQueue})
			if err != nil {
				return err
			}

			// Cleanup the resources when the streaming process terminates
			execCleanup(h, id)
			return nil
		}

		if err = t.Safe(fn); err != nil {
			h.Log.Error(ErrExecTerminal(err))
			execCleanup(h, id)

			// If the TTY fails then send the error message to the client
			if err = h.Broker.Publish(id, &broker.Message{
				ObjectType: broker.ErrorObject,
				Object:     err.Error(),
			}); err != nil {
				h.Log.Error(ErrExecTerminal(err))
			}

			return
		}
	}()

	// TTY stdout streaming Goroutine
	go func() {
		rdr := bufio.NewReader(getStdout)
		for {
			data := make([]byte, 1*KB)
			_, err = rdr.Read(data)
			if err == io.EOF {
				break // No clean up here as this can generate a false positive
			}

			err = h.Broker.Publish(id, &broker.Message{
				ObjectType: broker.ExecOutputObject,
				Object:     string(data),
			})
			if err != nil {
				h.Log.Error(ErrExecTerminal(err))
			}
		}
	}()

	for {
		if _, ok := h.channelPool[id]; !ok {
			h.Log.Info("Session closed for: ", id)
			return
		}

		select {
		case msg := <-subCh:
			if msg.ObjectType == broker.ExecInputObject {
				_, err = io.CopyBuffer(putStdin, strings.NewReader(msg.Object.(string)+"\n"), nil)
				if err != nil {
					h.Log.Error(ErrExecTerminal(err))
				}
			}
		case <-h.channelPool[id].(channels.StructChannel):
			h.Log.Info("Closing", id)
			delete(h.channelPool, id)
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

	structChan <- struct{}{}
}

func generateID() string {
	return uuid.New().String()
}
