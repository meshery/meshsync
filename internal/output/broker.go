package output

import (
	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
)

type BrokerWriter struct {
	br broker.Handler
}

func NewBrokerWriter(br broker.Handler) *BrokerWriter {
	return &BrokerWriter{
		br: br,
	}
}

func (s *BrokerWriter) Write(
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
