package meshsync

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/meshsync/internal/channels"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
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
	defer func() {
		if !utils.IsClosed(pipelineCh) {
			h.Log.Info("Closing informer stop channel")
			close(pipelineCh)
		}
	}()

	go h.startDiscovery(pipelineCh)

	debouncedStartDiscovery := debounce(time.Second*5, func(pipelinechannel chan struct{}) {
		if !utils.IsClosed[struct{}](pipelinechannel) {
			h.Log.Info("closing previous instance ")
			close(pipelinechannel)
		}
		pipelineCh = make(chan struct{})

		err := h.UpdateInformer()
		if err != nil {
			h.Log.Error(err)
		}
		h.Log.Info("starting over")
		h.startDiscovery(pipelineCh)

	})
loop:
	for {
		select {
		case <-h.channelPool[channels.Stop].(channels.StopChannel):
			break loop
		case <-h.channelPool[channels.ReSync].(channels.ReSyncChannel):
			go debouncedStartDiscovery(pipelineCh)
		}
	}
	h.Log.Info("Stopping Run")
}

func (h *Handler) UpdateInformer() error {
	dynamicClient, err := dynamic.NewForConfig(&h.kubeClient.RestConfig)
	if err != nil {
		return ErrNewInformer(err)
	}
	listOptionsFunc, err := GetListOptionsFunc(h.Config)
	if err != nil {
		return err
	}
	if h.informer != nil {
		h.informer.Shutdown()
	}
	h.informer = GetDynamicInformer(h.Config, dynamicClient, listOptionsFunc)
	return nil
}

func (h *Handler) ShutdownInformer() {
	if h.informer != nil {
		h.Log.Info("Shutting down informer...")
		h.informer.Shutdown()
		h.Log.Info("Shutting down informer done.")
	}
}

// TODO fix cyclop error
// Error: meshsync/handlers.go:71:1: calculated cyclomatic complexity for function ListenToRequests is 19, max is 10 (cyclop)
//
//nolint:cyclop
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

	processRequest := func(request *broker.Message) {
		if request.Request == nil {
			h.Log.Error(ErrInvalidRequest)
			return
		}

		switch request.Request.Entity {
		case broker.LogRequestEntity:
			h.Log.Info("Starting log session")
			err := h.processLogRequest(request.Request.Payload, listenerConfigs[config.LogStream])
			if err != nil {
				h.Log.Error(err)
				return
			}

			// TODO: Add this to the broker pkg
		case "informer-store":
			d, err := json.Marshal(request.Request.Payload)
			// TODO: Update broker pkg in Meshkit to include Reply types
			var payload struct{ Reply string }
			if err != nil {
				h.Log.Error(err)
				return
			}
			err = json.Unmarshal(d, &payload)
			if err != nil {
				h.Log.Error(err)
				return
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
				return
			}
		case broker.ActiveExecEntity:
			h.Log.Info("Connecting to channel pool")
			err := h.processActiveExecRequest()
			if err != nil {
				h.Log.Error(err)
				return
			}
		case "meshsync-meta":
			h.Log.Info("Publishing MeshSync metadata to the subject")
			err := h.Broker.Publish("meshsync-meta", &broker.Message{
				Object: config.Server["version"],
			})
			if err != nil {
				h.Log.Error(err)
				return
			}
		}
	}

loop:
	for {
		select {
		case <-h.channelPool[channels.Stop].(channels.StopChannel):
			break loop
		case request := <-reqChan:
			processRequest(request)
		}
	}
	h.Log.Info("Stopping ListenToRequests")
}

func (h *Handler) listStoreObjects() []model.KubernetesResource {
	objects := make([]interface{}, 0)
	for _, v := range h.stores {
		objects = append(objects, v.List()...)
	}
	parsedObjects := make([]model.KubernetesResource, 0)
	for _, obj := range objects {
		parsedObjects = append(
			parsedObjects,
			model.ParseList(
				*obj.(*unstructured.Unstructured),
				broker.Add,
				h.clusterID,
			),
		)
	}
	return parsedObjects
}

// TODO
// fix lint error
// calculated cyclomatic complexity for function WatchCRDs is 11, max is 10 (cyclop)
//
//nolint:cyclop
func (h *Handler) WatchCRDs() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	crdWatcher, err := h.kubeClient.DynamicKubeClient.Resource(schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}).Watch(ctx, metav1.ListOptions{})

	if err != nil {
		h.Log.Error(err)
		return
	}

	processEvent := func(event watch.Event) {
		crd := &kubernetes.CRDItem{}
		if event.Object == nil {
			// TODO
			// https://github.com/meshery/meshsync/issues/434
			h.Log.Info("Handler::WatchCRDs::processEvent event.Object is nil, skipping")
			return
		}
		byt, err := json.Marshal(event.Object)
		if err != nil {
			h.Log.Error(err)
			return
		}

		err = json.Unmarshal(byt, crd)
		if err != nil {
			h.Log.Error(err)
			return
		}

		if len(crd.Spec.Versions) == 0 {
			h.Log.Warnf(
				"Handler::WatchCRDs::processEvent: event.Object has empty spec.Versions [%s]",
				string(byt),
			)
		}

		gvr := tmpGetGVRForCustomResources(crd)

		existingPipelines := config.Pipelines
		err = h.Config.GetObject(config.ResourcesKey, existingPipelines)
		if err != nil {
			h.Log.Error(err)
			return
		}

		existingPipelineConfigs := existingPipelines[config.GlobalResourceKey]

		configName := fmt.Sprintf("%s.%s.%s", gvr.Resource, gvr.Version, gvr.Group)
		updatedPipelineConfigs := existingPipelineConfigs

		switch event.Type {
		case watch.Added:
			// No need to verify if config is already added because If the config already exists then it indicates the informer has already synced that resource.
			// Any subsequent updates will have event type as "modified"
			updatedPipelineConfigs = existingPipelineConfigs.Add(config.PipelineConfig{
				Name:      configName,
				PublishTo: config.DefaultPublishingSubject,
				Events:    []string{"ADDED", "MODIFIED", "DELETED"},
			})
		case watch.Deleted:
			updatedPipelineConfigs = existingPipelineConfigs.Delete(config.PipelineConfig{
				Name: configName,
			})
		}
		existingPipelines[config.GlobalResourceKey] = updatedPipelineConfigs
		err = h.Config.SetObject(config.ResourcesKey, existingPipelines)
		if err != nil {
			h.Log.Error(err)
			h.Log.Info("skipping informer resync")
			return
		}
		h.Log.Info("Resyncing informer from watch crd")
		h.channelPool[channels.ReSync].(channels.ReSyncChannel).ReSyncInformer()
	}

loop:
	for {
		select {
		case <-h.channelPool[channels.Stop].(channels.StopChannel):
			break loop
		case event := <-crdWatcher.ResultChan():
			processEvent(event)
		}
	}
	h.Log.Info("Stopping WatchCRDs")
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

// TODO: this is temp fix, original is here
// https://github.com/meshery/meshkit/blob/master/utils/kubernetes/crd.go#L49C6-L49C30
// it is panics if crd.Spec.Versions is empty
func tmpGetGVRForCustomResources(crd *kubernetes.CRDItem) *schema.GroupVersionResource {
	if crd == nil {
		return nil
	}

	if len(crd.Spec.Versions) > 0 {
		return kubernetes.GetGVRForCustomResources(crd)
	}

	return &schema.GroupVersionResource{
		Group:    crd.Spec.Group,
		Version:  "",
		Resource: crd.Spec.Names.ResourceName,
	}
}
