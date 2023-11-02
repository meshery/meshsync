// Copyright 2023 Layer5, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package meshsync

import (
	"github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/internal/pipeline"
	"k8s.io/client-go/tools/cache"
)

func (h *Handler) startDiscovery(pipelineCh chan struct{}) {
	pipelineConfigs := make(map[string]config.PipelineConfigs, 10)
	err := h.Config.GetObject(config.ResourcesKey, &pipelineConfigs)
	if err != nil {
		h.Log.Error(ErrGetObject(err))
	}

	h.Log.Info("Pipeline started")
	pl := pipeline.New(h.Log, h.informer, h.Broker, pipelineConfigs, pipelineCh, h.dynamicClient, h.Config)
	result := pl.Run()
	h.stores = result.Data.(map[string]cache.Store)
	if result.Error != nil {
		h.Log.Error(ErrNewPipeline(result.Error))
	}
}
