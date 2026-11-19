// Package discovery defines the platform-agnostic seam between a platform's
// resource discovery (Kubernetes today; Docker and public clouds later) and
// MeshSync's output plumbing (broker, file, deduplication).
//
// Each platform implements Discoverer in its own subpackage, for example
// internal/discovery/kubernetes. A platform owns how it fans out internally
// (Kubernetes registers one informer per resource kind); everything it finds is
// handed to the EmitFunc as a model.Resource.
package discovery

import (
	"context"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/internal/output"
	"github.com/meshery/meshsync/pkg/model"
)

// Discoverer discovers and continuously synchronizes the resources of one
// platform scope (a Kubernetes cluster, a Docker daemon, a cloud account).
type Discoverer interface {
	// Platform returns a stable, lowercase platform discriminator such as
	// "kubernetes". It is used as model.Resource.Platform.
	Platform() string

	// ScopeID returns the identity of the scope this Discoverer watches: a
	// Kubernetes cluster UID, a Docker daemon ID, a cloud account and region.
	// It must be stable across MeshSync restarts against the same target.
	ScopeID(ctx context.Context) (string, error)

	// Discover performs one full inventory pass, calling emit for every
	// resource currently visible. Platforms whose Watch already lists
	// everything before watching (Kubernetes informers) may report that a
	// standalone pass is not supported.
	Discover(ctx context.Context, emit EmitFunc) error

	// Watch emits every Add, Update and Delete until stopCh is closed, and
	// blocks until it has stopped. Platforms without a native watch API
	// implement it as a poll-and-diff loop.
	Watch(ctx context.Context, stopCh <-chan struct{}, emit EmitFunc) error

	// Shutdown releases any resources the Discoverer still holds. It must be
	// safe to call more than once and after Watch has returned.
	Shutdown()
}

// EmitFunc receives every resource a Discoverer finds or sees change. cfg is
// the pipeline configuration the resource was discovered under; output writers
// use it to pick the publish subject.
type EmitFunc func(res model.Resource, evtype broker.EventType, cfg config.PipelineConfig) error

// WriterEmitFunc adapts an output.Writer into an EmitFunc.
//
// The writer receives the embedded Kubernetes payload exactly as discovered, so
// the broker messages and snapshot files it produces are unchanged. Resources
// without a Kubernetes payload are rejected until the output writers accept the
// platform-agnostic envelope.
func WriterEmitFunc(w output.Writer) EmitFunc {
	return func(res model.Resource, evtype broker.EventType, cfg config.PipelineConfig) error {
		if res.Kubernetes == nil {
			return ErrUnsupportedResource(res.Platform, res.Kind)
		}
		return w.Write(*res.Kubernetes, evtype, cfg)
	}
}

// Factory constructs a new Discoverer.
type Factory func() (Discoverer, error)

// Registry maps platform names to Discoverer factories.
//
// A Registry holds factories, never Discoverer instances: Build constructs a
// fresh Discoverer on every call, so no discovery state is shared between runs.
// Create one per Run with NewRegistry; there is no package-level registry.
type Registry struct {
	factories map[string]Factory
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds the factory for a platform, replacing any earlier one.
func (r *Registry) Register(platform string, factory Factory) {
	r.factories[platform] = factory
}

// Platforms returns the registered platform names.
func (r *Registry) Platforms() []string {
	platforms := make([]string, 0, len(r.factories))
	for platform := range r.factories {
		platforms = append(platforms, platform)
	}
	return platforms
}

// Build constructs a new Discoverer for the given platform.
func (r *Registry) Build(platform string) (Discoverer, error) {
	factory, ok := r.factories[platform]
	if !ok || factory == nil {
		return nil, ErrUnknownPlatform(platform)
	}
	d, err := factory()
	if err != nil {
		return nil, ErrBuildDiscoverer(platform, err)
	}
	return d, nil
}
