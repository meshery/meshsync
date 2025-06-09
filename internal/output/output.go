package output

import (
	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
)

type Writer interface {
	Write(
		obj model.KubernetesResource,
		evtype broker.EventType,
		config config.PipelineConfig,
	) error
}
