// Package kubernetes is the Kubernetes discovery.Discoverer. It wraps the
// existing meshsync.Handler and internal/pipeline machinery without changing
// how Kubernetes discovery works: informers, resync, CRD watching and the
// broker request listeners all run exactly as before.
package kubernetes

import (
	"context"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/config"
	"github.com/meshery/meshkit/logger"
	mesherykube "github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/meshsync/internal/channels"
	internalconfig "github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/internal/discovery"
	"github.com/meshery/meshsync/internal/output"
	"github.com/meshery/meshsync/meshsync"
	"github.com/meshery/meshsync/pkg/model"
)

// Options holds what the Kubernetes Discoverer needs. The first group mirrors
// the arguments of meshsync.New.
type Options struct {
	Config           config.Handler
	KubeClient       *mesherykube.Client
	Log              logger.Handler
	Broker           broker.Handler
	OutputWriter     output.Writer
	ChannelPool      map[string]channels.GenericChannel
	OutputFiltration internalconfig.OutputFiltrationContainer

	// BlackList is the watch-list blacklist the CRD watch honours.
	BlackList []string
	// WatchCRDs starts the CRD watch alongside discovery (the MeshSync CRD
	// is present in the cluster).
	WatchCRDs bool
	// ListenToRequests starts the broker request listeners (broker output
	// mode only).
	ListenToRequests bool
}

// Discoverer discovers a Kubernetes cluster through meshsync.Handler.
type Discoverer struct {
	handler          *meshsync.Handler
	watchCRDs        bool
	listenToRequests bool
}

var _ discovery.Discoverer = (*Discoverer)(nil)

// New builds a Kubernetes Discoverer. It constructs the meshsync.Handler the
// same way pkg/lib/meshsync.Run did before the Discoverer existed.
func New(opts Options) (*Discoverer, error) {
	h, err := meshsync.New(
		opts.Config,
		opts.KubeClient,
		opts.Log,
		opts.Broker,
		opts.OutputWriter,
		opts.ChannelPool,
		opts.OutputFiltration,
	)
	if err != nil {
		return nil, err
	}
	h.BlackList = opts.BlackList

	return &Discoverer{
		handler:          h,
		watchCRDs:        opts.WatchCRDs,
		listenToRequests: opts.ListenToRequests,
	}, nil
}

// Platform returns model.PlatformKubernetes.
func (d *Discoverer) Platform() string {
	return model.PlatformKubernetes
}

// ScopeID returns the cluster ID (the kube-system namespace UID). It is empty
// when the cluster ID could not be read, matching the legacy cluster_id value.
func (d *Discoverer) ScopeID(_ context.Context) (string, error) {
	return d.handler.ClusterID(), nil
}

// Discover is not supported on its own: Kubernetes informers list every
// resource before they start watching, so the initial inventory is emitted by
// Watch as ADD events.
func (d *Discoverer) Discover(_ context.Context, _ discovery.EmitFunc) error {
	return discovery.ErrDiscoverNotSupported(d.Platform())
}

// Watch runs Kubernetes discovery and blocks until it stops.
//
// It starts the CRD watch and the broker request listeners when enabled, then
// runs meshsync.Handler.Run. The Handler stops when the channel pool's Stop
// channel closes, so stopCh must be that same channel; ctx is not used.
func (d *Discoverer) Watch(_ context.Context, _ <-chan struct{}, emit discovery.EmitFunc) error {
	d.handler.SetEmitFunc(emit)

	if d.watchCRDs {
		go d.handler.WatchCRDs()
	}
	if d.listenToRequests {
		// The listeners serve exec, log and informer-store requests over the
		// broker, so they only run when the output mode is broker.
		go d.handler.ListenToRequests()
	}

	d.handler.Run()
	return nil
}

// Shutdown is a no-op. Kubernetes teardown is driven by closing the channel
// pool's Stop channel: Handler.Run stops the discovery pipeline and shuts the
// informer factory down itself. Calling ShutdownInformer from here while Run is
// still tearing down would contend on the Handler's lifecycle lock.
func (d *Discoverer) Shutdown() {}
