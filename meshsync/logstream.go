package meshsync

import (
	"context"
	"fmt"
	"io"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
	v1 "k8s.io/api/core/v1"
)

func (h *Handler) processLogRequest(obj interface{}, cfg config.ListenerConfig) error {
	reqs := make(model.LogRequests)
	d, err := utils.Marshal(obj)
	if err != nil {
		return err
	}

	err = utils.Unmarshal(d, &reqs)
	for _, req := range reqs {
		id := fmt.Sprintf("%s:%s:%s", req.Namespace, req.Name, req.Container)
		if _, ok := h.channelPool[id]; !ok {
			// Subscribing the first time
			if !bool(req.Stop) {
				h.channelPool[id] = make(chan struct{})
				go h.streamLogs(id, req, cfg)
			}
		} else {
			// Already running subscription
			if bool(req.Stop) {
				h.channelPool[id] <- struct{}{}
			}
		}
	}

	return nil
}

func (h *Handler) streamLogs(id string, req model.LogRequest, cfg config.ListenerConfig) {
	resp, err := h.staticClient.CoreV1().Pods(req.Namespace).GetLogs(req.Name, &v1.PodLogOptions{
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

	for {
		buf := make([]byte, 2000)
		numBytes, err := resp.Read(buf)
		if numBytes == 0 {
			continue
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			h.Log.Error(ErrCopyBuffer(err))
		}

		message := string(buf[:numBytes])
		err = h.Broker.Publish(cfg.PublishTo, &broker.Message{
			ObjectType: broker.LogStreamObject,
			EventType:  broker.Add,
			Object: &model.LogObject{
				ID:   req.ID,
				Data: message,
			},
		})
		if err != nil {
			h.Log.Error(ErrCopyBuffer(err))
		}
	}

	select {
	case <-h.channelPool[id]:
		h.Log.Info("Closing", id)
		delete(h.channelPool, id)
		return
	}
}
