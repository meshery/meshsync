package meshsync

import (
	"time"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshsync/internal/channels"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (h *Handler) Run() {
	pipelineCh := make(chan struct{})
	go h.startDiscovery(pipelineCh)
	for range h.channelPool[channels.ReSync].(channels.ReSyncChannel) {
		go func(ch chan struct{}) {
			for {
				h.Log.Info("stopping previous instance")
				if _, ok := <-ch; ok {
					ch <- struct{}{}
				}
			}
		}(pipelineCh)
		h.Log.Info("starting over")
		pipelineCh = make(chan struct{})
		go h.startDiscovery(pipelineCh)
		time.Sleep(5 * time.Second)
	}
}

func (h *Handler) ListenToRequests() {
	listenerConfigs := make(map[string]config.ListenerConfig, 10)
	err := h.Config.GetObject(config.ListenersKey, &listenerConfigs)
	if err != nil {
		h.Log.Error(ErrGetObject(err))
	}

	h.Log.Info("Listening for requests in: ", listenerConfigs[config.RequestStream].SubscribeTo)
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

			// TODO: Add this to the broker pkg
		case "informer-store":

			allInformersCacheSynced := true
			// checks if all the SharedInformers cache are synced atleast once
			for _, v := range h.informer.WaitForCacheSync(make(<-chan struct{})) {
				if !v {
					allInformersCacheSynced = false
				}
			}

			d, err := utils.Marshal(request.Request.Payload)
			// TODO: Update broker pkg in Meshkit to include Reply types
			var payload struct{ Reply string }
			if err != nil {
				h.Log.Error(err)
				continue
			}
			err = utils.Unmarshal(d, &payload)
			if err != nil {
				h.Log.Error(err)
				continue
			}

			replySubject := payload.Reply

			if !allInformersCacheSynced {
				err = h.Broker.Publish(replySubject, &broker.Message{
					EventType: broker.ErrorEvent,
					Object:    "Stores have not been synced. Please try again after sometime",
				})
				if err != nil {
					h.Log.Error(err)
					continue
				}
				continue
			}

			h.Log.Info("Sending the current state of the informer store to ", replySubject)
			storeObjects := h.listStoreObjects()
			newList := make([]model.Object, 0)
			for _, obj := range storeObjects {
				newList = append(newList, model.ParseList(*obj.(*unstructured.Unstructured)))
			}

			splitSlices := splitIntoMultipleSlices(newList, 5)
			for _, val := range splitSlices {
				err = h.Broker.Publish(replySubject, &broker.Message{
					Object: val,
				})
				if err != nil {
					h.Log.Error(err)
					continue
				}
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
		case broker.ActiveExecEntity:
			h.Log.Info("Connecting to channel pool")
			err := h.processActiveExecRequest()
			if err != nil {
				h.Log.Error(err)
				continue
			}
		}
	}
}

func (h *Handler) listStoreObjects() []interface{} {
	objects := make([]interface{}, 0)
	for _, v := range h.stores {
		objects = append(objects, v.List()...)
	}
	return objects
}

// TODO: move this to meshkit
// given [1,2,3,4,5,6,7,5,4,4] and 3 as its arguments, it would
// return [[1,2,3], [4,5,6], [7,5,4], [4]]
func splitIntoMultipleSlices(s []model.Object, maxItmsPerSlice int) []([]model.Object) {
	result := make([]([]model.Object), 0)
	temp := make([]model.Object, 0)

	for idx, val := range s {
		temp = append(temp, val)
		if ((idx + 1) % maxItmsPerSlice) == 0 {
			result = append(result, temp)
			temp = nil
		}
		if idx+1 == len(s) {
			if len(temp) != 0 {
				result = append(result, temp)
			}
		}
	}

	return result
}
