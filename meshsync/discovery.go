// Copyright Meshery Authors
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
	"fmt"

	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/internal/pipeline"
	"k8s.io/client-go/tools/cache"
)

func (h *Handler) startDiscovery(pipelineCh chan struct{}) {
	pipelineConfigs := make(map[string]config.PipelineConfigs, 10)
	err := h.Config.GetObject(config.ResourcesKey, &pipelineConfigs)
	if err != nil {
		h.Log.Error(ErrGetObject(err))
		return
	}

	h.Log.Debug("Pipeline started")
	pl := pipeline.New(h.Log, h.informer, h.outputWriter, pipelineConfigs, pipelineCh, h.clusterID, h.outputFiltration)
	result := pl.Run()
	if result.Error != nil {
		h.Log.Error(ErrNewPipeline(result.Error))
		return
	}

	data, ok := result.Data.(map[string]cache.Store)
	if !ok || data == nil {
		// handle error: type mismatch or nil
		h.Log.Error(ErrNewPipeline(fmt.Errorf("unexpected type or nil data for result.Data")))
		return
	}

	h.replaceStores(data)
}

// replaceStores swaps in the per-GVR informer store set produced by a discovery
// run. stores is read concurrently by handleInformerStoreRequest, so the swap is
// guarded by storesMu.
func (h *Handler) replaceStores(data map[string]cache.Store) {
	h.storesMu.Lock()
	defer h.storesMu.Unlock()
	h.stores = data
}

// snapshotStores returns the current per-GVR informer stores. storesMu is held
// only while copying the map values, never across the caller's subsequent
// store reads, so a store List() never blocks the discovery goroutine's next
// replaceStores.
func (h *Handler) snapshotStores() []cache.Store {
	h.storesMu.RLock()
	defer h.storesMu.RUnlock()
	stores := make([]cache.Store, 0, len(h.stores))
	for _, s := range h.stores {
		stores = append(stores, s)
	}
	return stores
}
