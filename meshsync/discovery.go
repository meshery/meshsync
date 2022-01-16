package meshsync

import (
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/pipeline"
	// "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func (h *Handler) startDiscovery(pipelineCh chan struct{}) {
	pipelineConfigs := make(map[string]config.PipelineConfigs, 10)
	err := h.Config.GetObject(config.ResourcesKey, &pipelineConfigs)
	if err != nil {
		h.Log.Error(ErrGetObject(err))
	}

	h.Log.Info("Pipeline started")
	pl := pipeline.New(h.Log, h.informer, h.Broker, pipelineConfigs, pipelineCh, h.queue)
	result := pl.Run()
	h.stores = result.Data.(map[string]cache.Store)
	if result.Error != nil {
		h.Log.Error(ErrNewPipeline(result.Error))
	}
}
