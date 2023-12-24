package meshsync

import (
	"fmt"
	"time"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/kubernetes"
	"github.com/layer5io/meshsync/internal/channels"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

func debounce(d time.Duration, f func(ch chan struct{})) func(ch chan struct{}) {
	timer := time.NewTimer(d)
	return func(pipelineCh chan struct{}) {
		timer.Stop()
		timer = time.NewTimer(d)
		<-timer.C
		f(pipelineCh)
		timer.Reset(d)
		timer.Stop()
	}
}

func (h *Handler) Run() {
	pipelineCh := make(chan struct{})
	go h.startDiscovery(pipelineCh)

	debouncedStartDiscovery := debounce(time.Second*5, func(pipelinechannel chan struct{}) {
		go func() {
			// h.DeleteIndexers()
			h.UpdateInformer()
			h.startDiscovery(pipelinechannel)
		}()
	})
	for range h.channelPool[channels.ReSync].(channels.ReSyncChannel) {
		go func(ch chan struct{}) {

			if !utils.IsClosed[struct{}](ch) {
				close(ch)
			}
			h.Log.Info("starting over")
			pipelineCh = make(chan struct{})
			go debouncedStartDiscovery(pipelineCh)
		}(pipelineCh)
	}
}

func(h *Handler) UpdateInformer() error {
	dynamicClient, err := dynamic.NewForConfig(&h.restConfig)
	if err != nil {
		return ErrNewInformer(err)
	}
	listOptionsFunc, err := GetListOptionsFunc(h.Config)
	if err != nil {
		return err
	}
	h.informer = GetDynamicInformer(h.Config, dynamicClient, listOptionsFunc)
	return nil
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
			storeObjects := h.listStoreObjects()
			splitSlices := splitIntoMultipleSlices(storeObjects, 5) //  performance of NATS is bound to degrade if huge messages are sent

			h.Log.Info("Publishing the data from informer stores to the subject: ", replySubject)
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
		case "meshsync-meta":
			h.Log.Info("Publishing MeshSync metadata to the subject")
			err := h.Broker.Publish("meshsync-meta", &broker.Message{
				Object: config.Server["version"],
			})
			if err != nil {
				h.Log.Error(err)
				continue
			}
		}
	}
}

func (h *Handler) listStoreObjects() []model.KubernetesResource {
	objects := make([]interface{}, 0)
	for _, v := range h.stores {
		objects = append(objects, v.List()...)
	}
	parsedObjects := make([]model.KubernetesResource, 0)
	for _, obj := range objects {
		parsedObjects = append(parsedObjects, model.ParseList(*obj.(*unstructured.Unstructured)))
	}
	return parsedObjects
}

// TODO: move this to meshkit
// given [1,2,3,4,5,6,7,5,4,4] and 3 as its arguments, it would
// return [[1,2,3], [4,5,6], [7,5,4], [4]]
func splitIntoMultipleSlices(s []model.KubernetesResource, maxItmsPerSlice int) []([]model.KubernetesResource) {
	result := make([]([]model.KubernetesResource), 0)
	temp := make([]model.KubernetesResource, 0)

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
