package pipeline

import (
	"fmt"
	"strconv"

	"github.com/meshery/meshkit/broker"
	internalconfig "github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func (ri *RegisterInformer) GetEventHandlers() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			objCasted := obj.(*unstructured.Unstructured)
			err := ri.publishItem(objCasted, broker.Add, ri.config)
			if err != nil {
				ri.log.Error(err)
			}
			ri.log.Debugf("Received ADD event for: %s/%s of kind: %s", objCasted.GetName(), objCasted.GetNamespace(), objCasted.GroupVersionKind().Kind)
		},
		UpdateFunc: func(oldObj, obj interface{}) {
			oldObjCasted := oldObj.(*unstructured.Unstructured)
			objCasted := obj.(*unstructured.Unstructured)

			oldRV, _ := strconv.ParseInt(oldObjCasted.GetResourceVersion(), 0, 64)
			newRV, _ := strconv.ParseInt(objCasted.GetResourceVersion(), 0, 64)

			if oldRV < newRV {
				err := ri.publishItem(objCasted, broker.Update, ri.config)

				if err != nil {
					ri.log.Error(err)
				}
				ri.log.Debugf("Received UPDATE event for: %s/%s of kind: %s", objCasted.GetName(), objCasted.GetNamespace(), objCasted.GroupVersionKind().Kind)
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
			// The obj can only be of two types: *unstructured.Unstructured or
			// cache.DeletedFinalStateUnknown. The latter is a tombstone the informer
			// delivers after a watch gap/resync when it missed the final delete state,
			// so its wrapped object may be `stale`.

			// refer 'https://pkg.go.dev/k8s.io/client-go/tools/cache#ResourceEventHandler.OnDelete'

			// Unwrap the tombstone first. Asserting *unstructured.Unstructured directly
			// on a cache.DeletedFinalStateUnknown would panic and crash this goroutine.
			if staleObj, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = staleObj.Obj
			}

			objCasted, ok := obj.(*unstructured.Unstructured)
			if !ok {
				ri.log.Errorf("RegisterInformer::DeleteFunc: unexpected object type %T in delete event; skipping", obj)
				return
			}

			err := ri.publishItem(objCasted, broker.Delete, ri.config)
			if err != nil {
				ri.log.Error(err)
			}
			ri.log.Debugf("Received DELETE event for: %s/%s of kind: %s", objCasted.GetName(), objCasted.GetNamespace(), objCasted.GroupVersionKind().Kind)
		},
	}
}

func (ri *RegisterInformer) registerHandlers(s cache.SharedIndexInformer) {
	s.AddEventHandler(ri.GetEventHandlers()) // nolint
}

func (ri *RegisterInformer) publishItem(obj *unstructured.Unstructured, evtype broker.EventType, config internalconfig.PipelineConfig) error {
	if obj == nil {
		// skip nik resource
		ri.log.Debug("RegisterInformer::publishItem: skipping nil resource for even type ", evtype)
		return nil
	}

	// if the event is not supported skip
	if !slices.Contains(ri.config.Events, string(evtype)) {
		return nil
	}
	k8sResource := model.ParseList(*obj, evtype, ri.clusterID)

	if ri.checkMustSkip(obj) {
		// skip this resource
		ri.log.Debugf("RegisterInformer::publishItem: skipping resource: %s/%s of kind: %s", obj.GetName(), obj.GetNamespace(), k8sResource.Kind)
		return nil

	}

	if err := ri.outputWriter.Write(
		k8sResource,
		evtype,
		config,
	); err != nil {
		ri.log.Error(ErrWriteOutput(config.Name, err))
		return err
	}

	return nil
}

func (ri *RegisterInformer) checkMustSkip(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return true
	}

	conditions := []func() bool{
		func() bool {
			return len(ri.outputFiltration.NamespaceSet) > 0 &&
				!ri.outputFiltration.NamespaceSet.Contains(obj.GetNamespace())
		},
	}

	for _, condition := range conditions {
		if condition() {
			return true
		}
	}

	return false
}
