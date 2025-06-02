package output

import (
	"errors"

	"github.com/meshery/meshkit/broker"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
)

// a wrapper which allows to have multiple writers under one entity
type CompositeWriter struct {
	writersPool []Writer
}

func NewCompositeWriter(writer ...Writer) *CompositeWriter {
	return &CompositeWriter{
		writersPool: writer,
	}
}

func (w *CompositeWriter) Write(
	obj model.KubernetesResource,
	evtype broker.EventType,
	config config.PipelineConfig,
) error {
	errs := make([]error, 0, len(w.writersPool))

	for _, writer := range w.writersPool {
		if err := writer.Write(obj, evtype, config); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
