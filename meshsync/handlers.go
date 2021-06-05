package meshsync

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/pipeline"
)

func (h *Handler) Run(stopCh chan struct{}) {
	pipelineConfigs := make(map[string]config.PipelineConfigs, 10)
	err := h.Config.GetObject(config.ResourcesKey, &pipelineConfigs)
	if err != nil {
		h.Log.Error(ErrGetObject(err))
	}

	h.Log.Info("Pipeline started")
	pl := pipeline.New(h.Log, h.informer, h.Broker, pipelineConfigs, stopCh)
	result := pl.Run()
	if result.Error != nil {
		h.Log.Error(ErrNewPipeline(result.Error))
	}
}

func (h *Handler) ListenToRequests(stopCh chan struct{}) {
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
		h.Log.Info("Incoming Request")
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
		}
	}
}
