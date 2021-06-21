package meshsync

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

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
		id := fmt.Sprintf("exec.%s.%s.%s", req.Namespace, req.Name, req.Container)
		if _, ok := h.channelPool[id]; !ok {
			// Subscribing the first time
			if !bool(req.Stop) {
				h.channelPool[id] = channels.NewStructChannel()
				h.Log.Info("Starting session")
				go h.streamSession(id, req, cfg)
			}
		} else {
			// Already running subscription
			if bool(req.Stop) {
				h.channelPool[id].(channels.StructChannel) <- struct{}{}
			}
		}
	}

	return nil
}

func (h *Handler) streamSession(id string, req model.ExecRequest, cfg config.ListenerConfig) {
	subCh := make(chan *broker.Message)
	//stdin := os.Stdin
	stdin := &bytes.Buffer{}
	tstdin, putStdin := io.Pipe()
	//stdout := os.Stdout
	//stdout := &bytes.Buffer{}
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
		message := ""
		for {
			run, _, err := rdr.ReadRune()
			if err == io.EOF {
				break
			}
			if err != nil {
				h.Log.Error(ErrCopyBuffer(err))
			}

			message = message + string(run)
			if run == '#' {
				fmt.Println("stdout: ", message)
				err = h.Broker.Publish(id, &broker.Message{
					ObjectType: broker.ExecOutputObject,
					Object:     message,
				})
				if err != nil {
					h.Log.Error(ErrExecTerminal(err))
				}
			}
		}
	}()

	go func() {
		rdr := bufio.NewReader(tstdin)
		message := ""
		for {
			run, _, err := rdr.ReadRune()
			fmt.Printf("rune: %c", run)
			if err == io.EOF {
				break
			}
			if err != nil {
				h.Log.Error(ErrCopyBuffer(err))
			}
			if run == 0x0D {
				message = message + string(run)
				fmt.Println("Stdin: ", message)
			}
		}
	}()

	h.Log.Info(id)

	for {
		select {
		case msg := <-subCh:
			if msg.ObjectType == broker.ExecInputObject {
				fmt.Println("object: ", msg.Object)
				_, err = putStdin.Write([]byte(msg.Object.(string)))
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
