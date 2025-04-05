package file

import (
	"io"
	"os"

	"sigs.k8s.io/yaml"
)

type YAMLWriter struct {
	filename    string
	writeCloser io.WriteCloser
}

func NewYAMLWriter(filename string) (*YAMLWriter, error) {
	descriptor, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &YAMLWriter{
		filename:    filename,
		writeCloser: io.WriteCloser(descriptor),
	}, nil
}

var delimiter = []byte{'-', '-', '-', '\n'}

func (w *YAMLWriter) Write(data any) (int, error) {
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return 0, err
	}

	bytes = append(bytes, delimiter...)
	return w.writeCloser.Write(bytes)
}

func (w *YAMLWriter) Close() error {
	return w.writeCloser.Close()
}
