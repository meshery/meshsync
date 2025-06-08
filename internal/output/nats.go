package output

import (
	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
)

type NatsWriter struct {
	br broker.Handler
}

func NewNatsWriter(br broker.Handler) *NatsWriter {
	return &NatsWriter{
		br: br,
	}
}

func (s *NatsWriter) Write(
	obj model.KubernetesResource,
	evtype broker.EventType,
	config config.PipelineConfig,
) error {
	return s.br.Publish(
		config.PublishTo,
		&broker.Message{
			ObjectType: broker.MeshSync,
			EventType:  evtype,
			Object:     obj,
		},
	)
}
