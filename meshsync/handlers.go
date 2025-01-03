package meshsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	for range h.channelPool[channels.ReSync].(channels.ReSyncChannel) {
		go debouncedStartDiscovery(pipelineCh)
	}
}

func (h *Handler) UpdateInformer() error {
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

// func (h *Handler) ListenToRequests() {
// 	listenerConfigs := make(map[string]config.ListenerConfig, 10)
// 	err := h.Config.GetObject(config.ListenersKey, &listenerConfigs)
// 	if err != nil {
// 		h.Log.Error(ErrGetObject(err))
// 	}

// 	h.Log.Info("Listening for requests in: ", listenerConfigs[config.RequestStream].SubscribeTo)
// 	reqChan := make(chan *broker.Message)
// 	err = h.Broker.SubscribeWithChannel(listenerConfigs[config.RequestStream].SubscribeTo, listenerConfigs[config.RequestStream].ConnectionName, reqChan)
// 	if err != nil {
// 		h.Log.Error(ErrSubscribeRequest(err))
// 	}

// 	for request := range reqChan {
// 		if request.Request == nil {
// 			h.Log.Error(ErrInvalidRequest)
// 			continue
// 		}

// 		switch request.Request.Entity {
// 		case broker.LogRequestEntity:
// 			h.Log.Info("Starting log session")
// 			err := h.processLogRequest(request.Request.Payload, listenerConfigs[config.LogStream])
// 			if err != nil {
// 				h.Log.Error(err)
// 				continue
// 			}

// 			// TODO: Add this to the broker pkg
// 		case "informer-store":
// 			d, err := json.Marshal(request.Request.Payload)
// 			// TODO: Update broker pkg in Meshkit to include Reply types
// 			var payload struct{ Reply string }
// 			if err != nil {
// 				h.Log.Error(err)
// 				continue
// 			}
// 			err = json.Unmarshal(d, &payload)
// 			if err != nil {
// 				h.Log.Error(err)
// 				continue
// 			}
// 			replySubject := payload.Reply
// 			storeObjects := h.listStoreObjects()
// 			splitSlices := splitIntoMultipleSlices(storeObjects, 5) //  performance of NATS is bound to degrade if huge messages are sent

// 			h.Log.Info("Publishing the data from informer stores to the subject: ", replySubject)
// 			for _, val := range splitSlices {
// 				err = h.Broker.Publish(replySubject, &broker.Message{
// 					Object: val,
// 				})
// 				if err != nil {
// 					h.Log.Error(err)
// 					continue
// 				}
// 			}

// 		case broker.ReSyncDiscoveryEntity:
// 			h.Log.Info("Resyncing")
// 			h.channelPool[channels.ReSync].(channels.ReSyncChannel) <- struct{}{}
// 		case broker.ExecRequestEntity:
// 			h.Log.Info("Starting interactive session")
// 			err := h.processExecRequest(request.Request.Payload, listenerConfigs[config.ExecShell])
// 			if err != nil {
// 				h.Log.Error(err)
// 				continue
// 			}
// 		case broker.ActiveExecEntity:
// 			h.Log.Info("Connecting to channel pool")
// 			err := h.processActiveExecRequest()
// 			if err != nil {
// 				h.Log.Error(err)
// 				continue
// 			}
// 		case "meshsync-meta":
// 			h.Log.Info("Publishing MeshSync metadata to the subject")
// 			err := h.Broker.Publish("meshsync-meta", &broker.Message{
// 				Object: config.Server["version"],
// 			})
// 			if err != nil {
// 				h.Log.Error(err)
// 				continue
// 			}
// 		}
// 	}
// }

// Update the ListenToRequests function
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

		case "informer-store":
			d, err := json.Marshal(request.Request.Payload)
			var payload struct{ Reply string }
			if err != nil {
				h.Log.Error(err)
				continue
			}
			err = json.Unmarshal(d, &payload)
			if err != nil {
				h.Log.Error(err)
				continue
			}
			replySubject := payload.Reply
			storeObjects := h.listStoreObjects()
			splitSlices := splitIntoMultipleSlices(storeObjects, 5)

			h.Log.Info("Writing the data from informer stores to the file")
			for _, val := range splitSlices {
				err = writeToFile(replySubject, val) // Call the new function to write to file
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
			h.Log.Info("Writing MeshSync metadata to the file")
			err := writeToFile("meshsync-meta", config.Server["version"]) // Call the new function to write to file
			if err != nil {
				h.Log.Error(err)
				continue
			}
		}
	}
}

// Function to write data to a file
func writeToFile(filename string, data interface{}) error {
	filePath := filepath.Join("data", filename+".json") // Define the file path and name
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func (h *Handler) listStoreObjects() []model.KubernetesResource {
	objects := make([]interface{}, 0)
	for _, v := range h.stores {
		objects = append(objects, v.List()...)
	}
	parsedObjects := make([]model.KubernetesResource, 0)
	for _, obj := range objects {
		parsedObjects = append(parsedObjects, model.ParseList(*obj.(*unstructured.Unstructured), broker.Add))
	}
	return parsedObjects
}

func (h *Handler) WatchCRDs() {
	kubeclient, err := kubernetes.New(nil)
	if err != nil {
		h.Log.Error(err)
		return
	}

	crdWatcher, err := kubeclient.DynamicKubeClient.Resource(schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}).Watch(context.Background(), metav1.ListOptions{})

	if err != nil {
		h.Log.Error(err)
		return
	}

	for event := range crdWatcher.ResultChan() {

		crd := &kubernetes.CRDItem{}
		byt, err := json.Marshal(event.Object)
		if err != nil {
			h.Log.Error(err)
			continue
		}

		err = json.Unmarshal(byt, crd)
		if err != nil {
			h.Log.Error(err)
			continue
		}

		gvr := kubernetes.GetGVRForCustomResources(crd)

		existingPipelines := config.Pipelines
		err = h.Config.GetObject(config.ResourcesKey, existingPipelines)
		if err != nil {
			h.Log.Error(err)
			continue
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
		h.channelPool[channels.ReSync].(channels.ReSyncChannel).ReSyncInformer()
	}
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
