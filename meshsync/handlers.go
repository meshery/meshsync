package meshsync

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/channels"
	"github.com/layer5io/meshsync/internal/config"
)

func (h *Handler) Run() {
	pipelineCh := make(chan struct{})
	go h.startDiscovery(pipelineCh)
	for range h.channelPool[channels.ReSync].(channels.ReSyncChannel) {
		close(pipelineCh)
		pipelineCh = make(chan struct{})
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
			err := h.processLogRequest(request.Request.Payload, listenerConfigs[config.LogStream])
			if err != nil {
				h.Log.Error(err)
				continue
			}
		case broker.ReSyncDiscoveryEntity:
			h.Log.Info("Resyncing")
			h.channelPool[channels.ReSync].(channels.ReSyncChannel) <- struct{}{}
		case broker.ExecRequestEntity:
			err := h.processExecRequest(request.Request.Payload, listenerConfigs[config.ExecShell])
			if err != nil {
				h.Log.Error(err)
				continue
			}
		}
	}
}
