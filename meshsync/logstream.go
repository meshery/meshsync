package meshsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/channels"
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
		if _, ok := h.channelPool[id]; !ok {
			// Subscribing the first time
			if !bool(req.Stop) {
				h.channelPool[id] = channels.NewStructChannel()
				go h.streamLogs(id, req, cfg)
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
		delete(h.channelPool, id)
		return
	}

	go func() {
		<-h.channelPool[id].(channels.StructChannel)
		h.Log.Info("Closing", id)
		delete(h.channelPool, id)
		resp.Close()
	}()

	for {
		buf := make([]byte, 2000)
		numBytes, err := resp.Read(buf)
		if err == io.EOF {
			break
		}
		if numBytes == 0 {
			continue
		}
		if err != nil {
			h.Log.Error(ErrCopyBuffer(err))
			delete(h.channelPool, id)
		}

		message := string(buf[:numBytes])
		err = h.Broker.Publish(cfg.PublishTo, &broker.Message{
			ObjectType: broker.LogStreamObject,
			EventType:  broker.Add,
			Object: &model.LogObject{
				ID:        req.ID,
				Data:      message,
				Primary:   req.Name,
				Secondary: req.Container,
			},
		})
		if err != nil {
			h.Log.Error(ErrCopyBuffer(err))
		}
	}

}
