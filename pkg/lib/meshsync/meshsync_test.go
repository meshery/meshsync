package meshsync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
)

// TestBrokerHost verifies that the host part of a broker URL is extracted
// correctly regardless of scheme, credentials (userinfo) or port.
func TestBrokerHost(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:     "host and port without scheme",
			input:    "host:4222",
			expected: "host",
		},
		{
			name:     "scheme prefixed",
			input:    "nats://host:4222",
			expected: "host",
		},
		{
			name:     "token as userinfo",
			input:    "nats://TOKEN@host:4222",
			expected: "host",
		},
		{
			name:     "user and password with tls scheme",
			input:    "tls://user:pass@host:4222",
			expected: "host",
		},
		{
			name:     "ipv6 literal",
			input:    "nats://[::1]:4222",
			expected: "::1",
		},
		{
			name:     "bare host",
			input:    "host",
			expected: "host",
		},
		{
			name:      "empty string",
			input:     "",
			expectErr: true,
		},
		{
			name:      "scheme without host",
			input:     "nats://",
			expectErr: true,
		},
		{
			name:      "invalid character in host",
			input:     "nats://ho st:4222",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			host, err := brokerHost(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got host %q", tc.input, host)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if host != tc.expected {
				t.Errorf("expected host %q for input %q, got %q", tc.expected, tc.input, host)
			}
		})
	}
}

const (
	testPrimarySnapshotName = "snapshot.yaml"
	testPodName             = "pod-1"
)

func newTestResource(kind, uid, name string) model.KubernetesResource {
	return model.KubernetesResource{
		Kind: kind,
		KubernetesResourceMeta: &model.KubernetesResourceObjectMeta{
			UID:  uid,
			Name: name,
		},
	}
}

func TestPrepareFileOutputWriterClosesPrimaryFile(t *testing.T) {
	dir := t.TempDir()
	primaryPath := filepath.Join(dir, testPrimarySnapshotName)

	writer, cleanup, err := prepareFileOutputWriter(Options{
		OutputFileName: primaryPath,
	})
	if err != nil {
		t.Fatalf("prepareFileOutputWriter: %v", err)
	}

	res1 := newTestResource("Pod", "uid-1", testPodName)
	res2 := newTestResource("Service", "uid-2", "svc-2")

	if err := writer.Write(res1, broker.Add, config.PipelineConfig{}); err != nil {
		t.Fatalf("write res1: %v", err)
	}
	if err := writer.Write(res1, broker.Update, config.PipelineConfig{}); err != nil {
		t.Fatalf("write duplicate res1: %v", err)
	}
	if err := writer.Write(res2, broker.Add, config.PipelineConfig{}); err != nil {
		t.Fatalf("write res2: %v", err)
	}

	cleanup()

	data, err := os.ReadFile(primaryPath)
	if err != nil {
		t.Fatalf("read primary output: %v", err)
	}
	content := string(data)
	if strings.Count(content, testPodName) != 1 {
		t.Errorf("expected 1 deduplicated %s entry, got:\n%s", testPodName, content)
	}
	if strings.Count(content, "svc-2") != 1 {
		t.Errorf("expected 1 svc-2 entry, got:\n%s", content)
	}

	res3 := newTestResource("ConfigMap", "uid-3", "cm-3")
	if err := writer.Write(res3, broker.Add, config.PipelineConfig{}); !errors.Is(err, os.ErrClosed) {
		t.Errorf("expected os.ErrClosed after cleanup, got %v", err)
	}
}

func TestPrepareFileOutputWriterWithExtendedFile(t *testing.T) {
	dir := t.TempDir()
	primaryPath := filepath.Join(dir, testPrimarySnapshotName)
	extendedPath := filepath.Join(dir, "snapshot-extended.yaml")

	writer, cleanup, err := prepareFileOutputWriter(Options{
		OutputFileName:     primaryPath,
		OutputExtendedFile: true,
	})
	if err != nil {
		t.Fatalf("prepareFileOutputWriter: %v", err)
	}

	res := newTestResource("Pod", "uid-1", testPodName)
	if err := writer.Write(res, broker.Add, config.PipelineConfig{}); err != nil {
		t.Fatalf("write add: %v", err)
	}
	if err := writer.Write(res, broker.Update, config.PipelineConfig{}); err != nil {
		t.Fatalf("write update: %v", err)
	}

	cleanup()

	primaryData, err := os.ReadFile(primaryPath)
	if err != nil {
		t.Fatalf("read primary output: %v", err)
	}
	extendedData, err := os.ReadFile(extendedPath)
	if err != nil {
		t.Fatalf("read extended output: %v", err)
	}
	if strings.Count(string(primaryData), testPodName) != 1 {
		t.Errorf("expected 1 entry in primary output, got:\n%s", string(primaryData))
	}
	if strings.Count(string(extendedData), testPodName) != 2 {
		t.Errorf("expected 2 entries in extended output, got:\n%s", string(extendedData))
	}

	res2 := newTestResource("Service", "uid-2", "svc-2")
	if err := writer.Write(res2, broker.Add, config.PipelineConfig{}); !errors.Is(err, os.ErrClosed) {
		t.Errorf("expected os.ErrClosed after cleanup, got %v", err)
	}
}

type recordingWriteCloser struct {
	closeCount int
}

func (r *recordingWriteCloser) Write(_ any) (int, error) {
	return 0, nil
}

func (r *recordingWriteCloser) Close() error {
	r.closeCount++
	return nil
}

func TestPrepareFileOutputWriterClosesExtendedOnPrimaryError(t *testing.T) {
	dir := t.TempDir()
	primaryPath := filepath.Join(dir, testPrimarySnapshotName)
	extendedPath := filepath.Join(dir, "snapshot-extended.yaml")

	extendedRecorder := &recordingWriteCloser{}

	_, _, err := prepareFileOutputWriterWithOpener(
		Options{
			OutputFileName:     primaryPath,
			OutputExtendedFile: true,
		},
		func(name string) (yamlWriteCloser, error) {
			if name == extendedPath {
				return extendedRecorder, nil
			}
			return nil, os.ErrPermission
		},
	)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected error %v, got %v", os.ErrPermission, err)
	}
	if extendedRecorder.closeCount != 1 {
		t.Errorf("expected extended writer Close to be called once, got %d", extendedRecorder.closeCount)
	}
}
