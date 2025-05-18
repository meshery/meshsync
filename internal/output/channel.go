package output

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
)

type ChannelWriter struct {
	transport chan<- *ChannelItem
}

type ChannelItem = broker.Message

func NewChannelWriter(transport chan<- *ChannelItem) *ChannelWriter {
	return &ChannelWriter{
		transport: transport,
	}
}

func (s *ChannelWriter) Write(
	obj model.KubernetesResource,
	evtype broker.EventType,
	config config.PipelineConfig,
) error {
	s.transport <- &ChannelItem{
		ObjectType: broker.MeshSync,
		EventType:  evtype,
		Object:     obj,
	}

	return nil
}
