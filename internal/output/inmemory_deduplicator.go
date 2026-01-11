package output

import (
	"errors"
	"sync"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
)

// instead of direct write to output destination
// InMemoryDeduplicatorWriter collects data in memory identifying entity by metadata.uid
// and write to output only on program exit
type InMemoryDeduplicatorWriter struct {
	realWritter Writer

	mu sync.Mutex

	// for this entities for which model.KubernetesResource.KubernetesResourceMeta != nil
	storage map[string]*inMemoryDeduplicatorContainer
	// as model.KubernetesResource.KubernetesResourceMeta could be nil
	// treat such entities as unique
	// and just put them in this slice
	storageIfNoMetaUID []*inMemoryDeduplicatorContainer
}

func NewInMemoryDeduplicatorWriter(realWritter Writer) *InMemoryDeduplicatorWriter {
	return &InMemoryDeduplicatorWriter{
		realWritter:        realWritter,
		storage:            make(map[string]*inMemoryDeduplicatorContainer),
		storageIfNoMetaUID: make([]*inMemoryDeduplicatorContainer, 0, 128),
	}
}

func (w *InMemoryDeduplicatorWriter) Write(
	obj model.KubernetesResource,
	evtype broker.EventType,
	config config.PipelineConfig,
) error {
	uid := ""
	if obj.KubernetesResourceMeta != nil {
		uid = obj.KubernetesResourceMeta.UID
	}

	entity := &inMemoryDeduplicatorContainer{
		obj:    obj,
		evtype: evtype,
		config: config,
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if uid != "" {
		w.storage[uid] = entity
	} else {
		w.storageIfNoMetaUID = append(w.storageIfNoMetaUID, entity)
	}

	return nil
}

func (w *InMemoryDeduplicatorWriter) Flush() error {
	w.mu.Lock()
	// Quickly copy the data and reset the maps under lock.
	storageToFlush := w.storage
	storageIfNoMetaUIDToFlush := w.storageIfNoMetaUID
	w.storage = make(map[string]*inMemoryDeduplicatorContainer)
	w.storageIfNoMetaUID = make([]*inMemoryDeduplicatorContainer, 0, cap(w.storageIfNoMetaUID))
	w.mu.Unlock()

	errs := make([]error, 0, len(storageToFlush)+len(storageIfNoMetaUIDToFlush))

	// Perform slow I/O operations on the copied data without holding the lock.
	for _, v := range storageToFlush {
		if err := w.realWritter.Write(v.obj, v.evtype, v.config); err != nil {
			errs = append(errs, err)
		}
	}

	for _, v := range storageIfNoMetaUIDToFlush {
		if err := w.realWritter.Write(v.obj, v.evtype, v.config); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

type inMemoryDeduplicatorContainer struct {
	obj    model.KubernetesResource
	evtype broker.EventType
	config config.PipelineConfig
}
