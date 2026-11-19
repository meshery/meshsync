package discovery

import (
	"context"
	"testing"

	"github.com/meshery/meshkit/broker"
	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubDiscoverer struct{ platform string }

func (s *stubDiscoverer) Platform() string                                       { return s.platform }
func (s *stubDiscoverer) ScopeID(context.Context) (string, error)                { return "scope", nil }
func (s *stubDiscoverer) Discover(context.Context, EmitFunc) error               { return nil }
func (s *stubDiscoverer) Watch(context.Context, <-chan struct{}, EmitFunc) error { return nil }
func (s *stubDiscoverer) Shutdown()                                              {}

type recordingWriter struct {
	written []model.KubernetesResource
	events  []broker.EventType
	configs []config.PipelineConfig
}

func (w *recordingWriter) Write(obj model.KubernetesResource, evtype broker.EventType, cfg config.PipelineConfig) error {
	w.written = append(w.written, obj)
	w.events = append(w.events, evtype)
	w.configs = append(w.configs, cfg)
	return nil
}

func TestRegistryBuildUnknownPlatform(t *testing.T) {
	_, err := NewRegistry().Build("docker")

	require.Error(t, err)
	assert.Equal(t, ErrUnknownPlatformCode, meshkiterrors.GetCode(err))
}

func TestRegistryBuildReturnsFreshInstance(t *testing.T) {
	r := NewRegistry()
	r.Register("stub", func() (Discoverer, error) { return &stubDiscoverer{platform: "stub"}, nil })

	first, err := r.Build("stub")
	require.NoError(t, err)
	second, err := r.Build("stub")
	require.NoError(t, err)

	assert.Equal(t, "stub", first.Platform())
	assert.NotSame(t, first, second, "Build must construct a new Discoverer on every call")
	assert.Equal(t, []string{"stub"}, r.Platforms())
}

func TestRegistryBuildWrapsFactoryError(t *testing.T) {
	r := NewRegistry()
	r.Register("stub", func() (Discoverer, error) { return nil, ErrUnknownPlatform("inner") })

	_, err := r.Build("stub")

	require.Error(t, err)
	assert.Equal(t, ErrBuildDiscovererCode, meshkiterrors.GetCode(err))
}

func TestWriterEmitFuncPassesKubernetesPayloadUnchanged(t *testing.T) {
	w := &recordingWriter{}
	kr := model.KubernetesResource{ID: "id-1", Kind: "Pod", ClusterID: "cluster-1"}
	cfg := config.PipelineConfig{Name: "pods.v1.", PublishTo: "meshery.meshsync.core"}

	err := WriterEmitFunc(w)(model.NewKubernetesResource(kr, "cluster-1"), broker.Update, cfg)

	require.NoError(t, err)
	require.Len(t, w.written, 1)
	assert.Equal(t, kr, w.written[0])
	assert.Equal(t, broker.Update, w.events[0])
	assert.Equal(t, cfg, w.configs[0])
}

func TestWriterEmitFuncRejectsNonKubernetesResource(t *testing.T) {
	w := &recordingWriter{}

	err := WriterEmitFunc(w)(model.Resource{Platform: "docker", Kind: "Container"}, broker.Add, config.PipelineConfig{})

	require.Error(t, err)
	assert.Equal(t, ErrUnsupportedResourceCode, meshkiterrors.GetCode(err))
	assert.Empty(t, w.written)
}
