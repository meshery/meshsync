package meshsync

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/pipeline"
)

func (h *Handler) Run() error {
	pipelineConfigs := make(map[string]config.PipelineConfigs, 10)
	err := h.Config.GetObject(config.ResourcesKey, &pipelineConfigs)
	if err != nil {
		return ErrGetObject(err)
	}

	stopCh := make(chan struct{})

	h.Log.Info("Pipeline started")
	pl := pipeline.New(h.Log, h.informer, h.Broker, pipelineConfigs, stopCh)
	result := pl.Run()
	if result.Error != nil {
		return ErrNewPipeline(result.Error)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, os.Interrupt)
	<-sigCh
	close(stopCh)

	return nil
}
