package output

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"
)

type Processor struct {
	output Writer
}

func NewProcessor() *Processor {
	return &Processor{}
}

func (p *Processor) SetOutput(output Writer) {
	p.output = output
}

func (p *Processor) Write(
	obj model.KubernetesResource,
	evtype broker.EventType,
	config config.PipelineConfig,
) error {
	return p.output.Write(obj, evtype, config)
}
