package output

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
)

type Writer interface {
	Write(
		obj model.KubernetesResource,
		evtype broker.EventType,
		config config.PipelineConfig,
	) error
}
