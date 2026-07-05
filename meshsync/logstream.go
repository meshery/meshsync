package meshsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	v1 "k8s.io/api/core/v1"
)

func (h *Handler) processLogRequest(obj interface{}, cfg config.ListenerConfig) error {
	reqs := make(model.LogRequests)
	d, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	err = json.Unmarshal(d, &reqs)
	if err != nil {
		return err
	}

	for _, req := range reqs {
		id := fmt.Sprintf("logs.%s.%s.%s", req.Namespace, req.Name, req.Container)
		if bool(req.Stop) {
			// Stop request: signal the running stream, if any, to close.
			// Non-blocking: the stream may already be closing and no longer
			// receiving, so a plain send could freeze this loop.
			if ch, ok := h.getSession(id); ok {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
			continue
		}
		if _, created := h.addSession(id); created {
			go h.streamLogs(id, req, cfg)
		}
	}

	return nil
}

func (h *Handler) streamLogs(id string, req model.LogRequest, cfg config.ListenerConfig) {
	// Remove the session however this function exits (stream-open failure, EOF,
	// read error, or a stop request).
	defer h.deleteSession(id)

	resp, err := h.kubeClient.KubeClient.CoreV1().Pods(req.Namespace).GetLogs(req.Name, &v1.PodLogOptions{
		Container:  req.Container,
		Follow:     req.Follow,
		Previous:   req.Previous,
		Timestamps: true,
		TailLines:  &req.TailLines,
		//SinceSeconds:
		//SinceTime:
		//LimitBytes:,
		//InsecureSkipTLSVerifyBackend: true,
	}).Stream(context.TODO())
	if err != nil {
		h.Log.Error(ErrLogStream(err))
		return
	}
	defer resp.Close()

	// done unblocks the waiter goroutine when the stream ends on its own (EOF or
	// read error), so it is not leaked once streamLogs returns.
	done := make(chan struct{})
	defer close(done)

	go func() {
		ch, ok := h.getSession(id)
		if !ok {
			return
		}
		select {
		case <-ch:
			// Stop request: close the stream so the read loop unblocks and exits.
			h.Log.Debugf("Closing %s", id)
			resp.Close()
		case <-done:
		}
	}()

	for {
		buf := make([]byte, 2000)
		numBytes, err := resp.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			// A non-EOF read error ends the stream; breaking avoids an infinite
			// read/log loop on a failed stream.
			h.Log.Error(ErrCopyBuffer(err))
			break
		}
		if numBytes == 0 {
			continue
		}

		message := string(buf[:numBytes])
		if pubErr := h.Broker.Publish(cfg.PublishTo, &broker.Message{
			ObjectType: broker.LogStreamObject,
			EventType:  broker.Add,
			Object: &model.LogObject{
				ID:        req.ID,
				Data:      message,
				Primary:   req.Name,
				Secondary: req.Container,
			},
		}); pubErr != nil {
			h.Log.Error(ErrCopyBuffer(pubErr))
		}
	}
}
