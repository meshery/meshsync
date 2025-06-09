package output

import (
	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/internal/file"
	"github.com/meshery/meshsync/pkg/model"
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
