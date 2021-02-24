package meshsync

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/config"
	"github.com/layer5io/meshkit/logger"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
)

// Handler contains all handlers, channels, clients, and other parameters for an adapter.
// Use type embedding in a specific adapter to extend it.
type Handler struct {
	Config config.Handler
	Log    logger.Handler
	Broker broker.Handler

	KubeClient *mesherykube.Client
}

func New(config config.Handler, log logger.Handler, broker broker.Handler) (*Handler, error) {
	// Initialize Kubeconfig
	kubeClient, err := mesherykube.New(nil)
	if err != nil {
		return nil, ErrKubeConfig(err)
	}

	return &Handler{
		Config: config,
		Log:    log,
		Broker: broker,

		KubeClient: kubeClient,
	}, nil
}
