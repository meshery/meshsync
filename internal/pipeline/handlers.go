package pipeline

import (
	"fmt"
	"strconv"

	"github.com/layer5io/meshkit/broker"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"
)

func (ri *RegisterInformer) GetEventHandlers() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			err := ri.publishItem(obj.(*unstructured.Unstructured), broker.Add, ri.config)
			if err != nil {
				ri.log.Error(err)
			}
			ri.log.Info("Received ADD event for: ", obj.(*unstructured.Unstructured).GetName(), "/", obj.(*unstructured.Unstructured).GetNamespace(), " of kind: ", obj.(*unstructured.Unstructured).GroupVersionKind().Kind)
		},
		UpdateFunc: func(oldObj, obj interface{}) {
			oldObjCasted := oldObj.(*unstructured.Unstructured)
			objCasted := obj.(*unstructured.Unstructured)

			oldRV, _ := strconv.ParseInt(oldObjCasted.GetResourceVersion(), 0, 64)
			newRV, _ := strconv.ParseInt(objCasted.GetResourceVersion(), 0, 64)

			if oldRV < newRV {
				err := ri.publishItem(obj.(*unstructured.Unstructured), broker.Update, ri.config)

				if err != nil {
					ri.log.Error(err)
				}
				ri.log.Info("Received UPDATE event for: ", obj.(*unstructured.Unstructured).GetName(), "/", obj.(*unstructured.Unstructured).GetNamespace(), " of kind: ", obj.(*unstructured.Unstructured).GroupVersionKind().Kind)
			} else {
				ri.log.Debug(fmt.Sprintf(
					"Skipping UPDATE event for: %s => [No changes detected]: %d %d",
					objCasted.GetName(),
					oldRV,
					newRV,
				))
			}
		},
		DeleteFunc: func(obj interface{}) {
			// the obj can only be of two types, Unstructured or DeletedFinalStateUnknown.
			// DeletedFinalStateUnknown means that the object that we receive may be `stale`
			// because of the way informer behaves

			// refer 'https://pkg.go.dev/k8s.io/client-go/tools/cache#ResourceEventHandler.OnDelete'

			var objCasted *unstructured.Unstructured
			objCasted = obj.(*unstructured.Unstructured)

			possiblyStaleObj, ok := obj.(cache.DeletedFinalStateUnknown)
			if ok {
				objCasted = possiblyStaleObj.Obj.(*unstructured.Unstructured)
			}
			err := ri.publishItem(objCasted, broker.Delete, ri.config)

			if err != nil {
				ri.log.Error(err)
			}
			ri.log.Info("Received DELETE event for: ", obj.(*unstructured.Unstructured).GetName(), "/", obj.(*unstructured.Unstructured).GetNamespace(), " of kind: ", obj.(*unstructured.Unstructured).GroupVersionKind().Kind)
		},
	}
}

func (ri *RegisterInformer) registerHandlers(s cache.SharedIndexInformer) {
	s.AddEventHandler(ri.GetEventHandlers()) // nolint
}

func (ri *RegisterInformer) publishItem(obj *unstructured.Unstructured, evtype broker.EventType, config internalconfig.PipelineConfig) error {

	// if the event is not supported skip
	if !slices.Contains(ri.config.Events, string(evtype)) {
		return nil
	}
	k8sResource := model.ParseList(*obj, evtype)
	if internalconfig.OutputMode == internalconfig.OutputModeNats {
		err := ri.broker.Publish(config.PublishTo, &broker.Message{
			ObjectType: broker.MeshSync,
			EventType:  evtype,
			Object:     k8sResource,
		})
		if err != nil {
			ri.log.Error(ErrPublish(config.Name, err))
			return err
		}
	}
	if internalconfig.OutputMode == internalconfig.OutputModeFile {
		// TODO move marshalling logic to file writer
		// data, err := json.Marshal(k8sResource)
		data, err := yaml.Marshal(k8sResource)
		if err != nil {
			ri.log.Error(ErrWriteFile(config.Name, err))
			return err
		}
		// data, err := yaml.Marshal(k8sResource)
		_, err = ri.fileWriter.Write(data)
		if err != nil {
			ri.log.Error(ErrWriteFile(config.Name, err))
			return err
		}
	}

	return nil
}
