package meshsync

import (
	"github.com/layer5io/meshsync/pkg/cluster"
	"github.com/layer5io/meshsync/pkg/meshes/istio"
)

func (h *Handler) StartDiscovery() error {

	err := cluster.Setup(h.DiscoveryClient, h.Broker, h.InformerClient)
	if err != nil {
		return ErrSetupCluster(err)
	}

	err = istio.Setup(h.DiscoveryClient, h.Broker, h.InformerClient)
	if err != nil {
		return ErrSetupIstio(err)
	}
	return nil
}
