package output

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
)

type Processor struct {
	strategy Writer
}

func NewProcessor() *Processor {
	return &Processor{}
}

func (p *Processor) SetStrategy(strategy Writer) {
	p.strategy = strategy
}

func (p *Processor) Write(
	obj model.KubernetesResource,
	evtype broker.EventType,
	config config.PipelineConfig,
) error {
	return p.strategy.Write(obj, evtype, config)
}
