package meshsync

import (
	"github.com/layer5io/meshery-adapter-library/config"
	"github.com/layer5io/meshkit/logger"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	"github.com/layer5io/meshsync/pkg/broker"
	"github.com/layer5io/meshsync/pkg/discovery"
	"github.com/layer5io/meshsync/pkg/informers"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Handler contains all handlers, channels, clients, and other parameters for an adapter.
// Use type embedding in a specific adapter to extend it.
type Handler struct {
	Config config.Handler
	Log    logger.Handler
	Broker broker.Handler

	RestConfig        rest.Config
	MesheryKubeclient *mesherykube.Client
	DiscoveryClient   *discovery.Client
	InformerClient    *informers.Client
	KubeClient        *kubernetes.Clientset
	DynamicKubeClient dynamic.Interface
}

func New(config config.Handler, log logger.Handler, broker broker.Handler) (*Handler, error) {

	// Initialize Kubeconfig
	rconfig, err := mesherykube.DetectKubeConfig()
	if err != nil {
		return nil, ErrKubeConfig(err)
	}

	// Configure discovery client
	dclient, err := discovery.NewClient(rconfig)
	if err != nil {
		return nil, ErrNewDiscovery(err)
	}

	// Configure informers client
	iclient, err := informers.NewClient(rconfig)
	if err != nil {
		return nil, ErrNewInformer(err)
	}

	// Configure kubeclient
	kclient, err := kubernetes.NewForConfig(rconfig)
	if err != nil {
		return nil, ErrNewKubeClient(err)
	}

	// Configure dynamic kubeclient
	dyclient, err := dynamic.NewForConfig(rconfig)
	if err != nil {
		return nil, ErrNewDynClient(err)
	}

	// Configure meshery kubeclient
	mclient, err := mesherykube.New(kclient, *rconfig)
	if err != nil {
		return nil, ErrNewMesheryClient(err)
	}

	return &Handler{
		Config: config,
		Log:    log,
		Broker: broker,

		RestConfig:        *rconfig,
		DiscoveryClient:   dclient,
		InformerClient:    iclient,
		MesheryKubeclient: mclient,
		KubeClient:        kclient,
		DynamicKubeClient: dyclient,
	}, nil
}
