package meshsync

import (
	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/config"
	"github.com/layer5io/meshkit/logger"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	"github.com/layer5io/meshsync/internal/channels"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Handler contains all handlers, channels, clients, and other parameters for an adapter.
// Use type embedding in a specific adapter to extend it.
type Handler struct {
	Config config.Handler
	Log    logger.Handler
	Broker broker.Handler

	restConfig   rest.Config
	informer     dynamicinformer.DynamicSharedInformerFactory
	staticClient *kubernetes.Clientset
	channelPool  map[string]channels.GenericChannel
}

func New(config config.Handler, log logger.Handler, br broker.Handler, pool map[string]channels.GenericChannel) (*Handler, error) {
	// Initialize Kubeconfig
	kubeClient, err := mesherykube.New(nil)
	if err != nil {
		return nil, ErrKubeConfig(err)
	}

	informer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(kubeClient.DynamicKubeClient, 0, v1.NamespaceAll, nil)

	return &Handler{
		Config:       config,
		Log:          log,
		Broker:       br,
		informer:     informer,
		restConfig:   kubeClient.RestConfig,
		staticClient: kubeClient.KubeClient,
		channelPool:  pool,
	}, nil
}
