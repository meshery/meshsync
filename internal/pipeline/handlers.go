package pipeline

import (
	"fmt"
	"strconv"

	"github.com/layer5io/meshkit/broker"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func (ri *RegisterInformer) registerHandlers(s cache.SharedIndexInformer) {
	handlers := cache.ResourceEventHandlerFuncs{
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
			newRV, _ := strconv.ParseInt(oldObjCasted.GetResourceVersion(), 0, 64)

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
	s.AddEventHandler(handlers)
}

func (ri *RegisterInformer) publishItem(obj *unstructured.Unstructured, evtype broker.EventType, config internalconfig.PipelineConfig) error {
	err := ri.broker.Publish(config.PublishTo, &broker.Message{
		ObjectType: broker.MeshSync,
		EventType:  evtype,
		Object:     model.ParseList(*obj),
	})
	if err != nil {
		ri.log.Error(ErrPublish(config.Name, err))
		return err
	}

	return nil
}
