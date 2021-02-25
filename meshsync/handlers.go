package meshsync

import (
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/informer"
	"github.com/layer5io/meshsync/internal/pipeline"
)

func (h *Handler) StartDiscovery() error {
	pipelineConfigs := make(map[string]config.PipelineConfigs, 10)
	err := h.Config.GetObject(config.ResourcesKey, &pipelineConfigs)
	if err != nil {
		return ErrGetObject(err)
	}

	h.Log.Info("Pipeline started")
	pl := pipeline.New(h.KubeClient.DynamicKubeClient, h.Broker, pipelineConfigs)
	result := pl.Run()
	if result.Error != nil {
		return ErrNewPipeline(result.Error)
	}

	return nil
}

func (h *Handler) StartInformers() error {
	informerConfigs := make(map[string]config.PipelineConfigs, 10)
	err := h.Config.GetObject(config.ResourcesKey, &informerConfigs)
	if err != nil {
		return ErrGetObject(err)
	}

	h.Log.Info("Informers started")
	err = informer.Run(h.KubeClient.DynamicKubeClient, h.Broker, informerConfigs)
	if err != nil {
		return ErrNewInformer(err)
	}

	interrupt := make(chan bool)
	for range interrupt {
		signal := <-interrupt
		if signal {
			return nil
		}
	}
	return nil
}
