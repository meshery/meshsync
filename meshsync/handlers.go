package meshsync

import (
	"context"
	"time"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/channels"
	"github.com/layer5io/meshsync/internal/config"
)

func (h *Handler) Run() {
	pipelineCh := make(chan struct{})
	go h.startDiscovery(pipelineCh)
	for range h.channelPool[channels.ReSync].(channels.ReSyncChannel) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		go func(c context.Context, cc context.CancelFunc) {
			for {
				h.Log.Info("stopping previous instance")
				select {
				case <-c.Done():
					return
				default:
					if _, ok := <-pipelineCh; ok {
						pipelineCh <- struct{}{}
					}
				}
			}
		}(ctx, cancel)
		h.Log.Info("waiting")
		time.Sleep(5 * time.Second)
		h.Log.Info("starting over")
		pipelineCh := make(chan struct{})
		go h.startDiscovery(pipelineCh)
	}
}

func (h *Handler) ListenToRequests() {
	listenerConfigs := make(map[string]config.ListenerConfig, 10)
	err := h.Config.GetObject(config.ListenersKey, &listenerConfigs)
	if err != nil {
		h.Log.Error(ErrGetObject(err))
	}

	h.Log.Info("Listening for requests")
	reqChan := make(chan *broker.Message)
	err = h.Broker.SubscribeWithChannel(listenerConfigs[config.RequestStream].SubscribeTo, listenerConfigs[config.RequestStream].ConnectionName, reqChan)
	if err != nil {
		h.Log.Error(ErrSubscribeRequest(err))
	}

	for request := range reqChan {
		if request.Request == nil {
			h.Log.Error(ErrInvalidRequest)
			continue
		}

		switch request.Request.Entity {
		case broker.LogRequestEntity:
			h.Log.Info("Starting log session")
			err := h.processLogRequest(request.Request.Payload, listenerConfigs[config.LogStream])
			if err != nil {
				h.Log.Error(err)
				continue
			}
		case broker.ReSyncDiscoveryEntity:
			h.Log.Info("Resyncing")
			h.channelPool[channels.ReSync].(channels.ReSyncChannel) <- struct{}{}
		case broker.ExecRequestEntity:
			h.Log.Info("Starting interactive session")
			err := h.processExecRequest(request.Request.Payload, listenerConfigs[config.ExecShell])
			if err != nil {
				h.Log.Error(err)
				continue
			}
		}
	}
}
