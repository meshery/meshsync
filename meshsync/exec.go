package meshsync

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshsync/internal/channels"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/util/interrupt"
	"k8s.io/kubectl/pkg/util/term"
)

func (h *Handler) processExecRequest(obj interface{}, cfg config.ListenerConfig) error {
	reqs := make(model.ExecRequests)
	d, err := utils.Marshal(obj)
	if err != nil {
		return err
	}

	err = utils.Unmarshal(d, &reqs)
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
				h.channelPool[id].(channels.StructChannel) <- struct{}{}

				err := h.Broker.Publish("active_sessions.exec", &broker.Message{
					ObjectType: broker.ActiveExecObject,
					Object:     h.getActiveChannels(),
				})
				if err != nil {
					h.Log.Error(ErrGetObject(err))
				}
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

func (h *Handler) streamChannelPool() error {
	go func() {
		for {
			err := h.Broker.Publish("active_sessions.exec", &broker.Message{
				ObjectType: broker.ActiveExecObject,
				Object:     h.getActiveChannels(),
			})
			if err != nil {
				h.Log.Error(ErrGetObject(err))
			}
		}
	}()

	return nil
}

func (h *Handler) streamSession(id string, req model.ExecRequest, cfg config.ListenerConfig) {
	subCh := make(chan *broker.Message)
	tstdin, putStdin := io.Pipe()
	stdin := ioutil.NopCloser(tstdin)
	getStdout, stdout := io.Pipe()
	err := h.Broker.SubscribeWithChannel(id, id, subCh)
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

			exec, err := remotecommand.NewSPDYExecutor(&h.restConfig, "POST", request.URL())
			if err != nil {
				return err
			}

			err = exec.Stream(remotecommand.StreamOptions{
				Stdin:             stdin,
				Stdout:            stdout,
				Stderr:            stdout,
				Tty:               true,
				TerminalSizeQueue: sizeQueue,
			})
			if err != nil {
				return err
			}
			return nil
		}

		if err := t.Safe(fn); err != nil {
			h.Log.Error(ErrExecTerminal(err))
			delete(h.channelPool, id)
			return
		}
	}()

	go func() {
		rdr := bufio.NewReader(getStdout)
		for {
			message, err := rdr.ReadString('#')
			if err == io.EOF {
				break
			}
			if err != nil {
				h.Log.Error(ErrCopyBuffer(err))
			}

			err = h.Broker.Publish(id, &broker.Message{
				ObjectType: broker.ExecOutputObject,
				Object:     message,
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
