package file

import (
	"errors"
	"io"
	"os"
)

var partSuffix = ".part"

type bufferedWriter struct {
	buffer         [][]byte
	filename       string
	descriptor     *os.File
	filenamePart   string
	descriptorPart *os.File
	writer         io.Writer
	writerPart     io.Writer
}

func NewBufferedWriter(outputFileName string) (*bufferedWriter, error) {
	filename := ""
	if outputFileName != "" {
		filename = outputFileName
	} else {
		fname, err := generateUniqueFileNameForSnapshot("json")
		if err != nil {
			return nil, err
		}
		filename = fname
	}

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	filenamePart := filename + partSuffix
	fPart, err := os.OpenFile(filenamePart, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &bufferedWriter{
		filename:       filename,
		descriptor:     f,
		filenamePart:   filenamePart,
		descriptorPart: fPart,
		writer:         io.Writer(f),
		writerPart:     io.Writer(fPart),
	}, nil
}

func (h *bufferedWriter) Write(data []byte) (int, error) {
	// append data to do a correct output in the end
	h.buffer = append(h.buffer, data)
	// but also write in a partial file
	h.writerPart.Write(data)
	// at least add a new line
	h.writerPart.Write([]byte{'\n'})
	return len(data), nil
}

// TODO
// this assumes the marshaling is in json
// revisit
func (h *bufferedWriter) FlushBuffer() (int, error) {
	data := []byte{'['}
	for i, item := range h.buffer {
		data = append(data, item...)
		if i != len(h.buffer)-1 {
			data = append(data, ',')
		}
	}
	data = append(data, ']')
	n, err := h.writer.Write(data)
	if err != nil {
		return n, err
	}

	return n, nil

}

func (h *bufferedWriter) Close() error {
	return errors.Join(
		h.descriptorPart.Close(),
		h.descriptor.Close(),
	)
}
