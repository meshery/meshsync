package output

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/file"
	"github.com/layer5io/meshsync/pkg/model"
)

type FileWriter struct {
	fw file.Writer
}

func NewFileWriter(fw file.Writer) *FileWriter {
	return &FileWriter{
		fw: fw,
	}
}

func (s *FileWriter) Write(
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
