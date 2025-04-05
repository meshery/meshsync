package output

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
)

type NatsStrategy struct {
	br broker.Handler
}

func NewNatsStrategy(br broker.Handler) *NatsStrategy {
	return &NatsStrategy{
		br: br,
	}
}

func (s *NatsStrategy) Write(
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
