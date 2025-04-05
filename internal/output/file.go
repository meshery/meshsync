package output

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/file"
	"github.com/layer5io/meshsync/pkg/model"
)

type FileStrategy struct {
	fw file.Writer
}

func NewFileStrategy(fw file.Writer) *FileStrategy {
	return &FileStrategy{
		fw: fw,
	}
}

func (s *FileStrategy) Write(
	obj model.KubernetesResource,
	evtype broker.EventType,
	config config.PipelineConfig,
) error {
	_, err := s.fw.Write(obj)
	if err != nil {
		return err
	}
	return nil
}
