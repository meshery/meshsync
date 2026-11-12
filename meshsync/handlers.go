package meshsync

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/meshsync/internal/channels"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

func (h *Handler) getSystemChannels() (channels.StopChannel, channels.ReSyncChannel) {
	var stopCh channels.StopChannel
	if rawStop, ok := h.channelPool[channels.Stop]; ok && rawStop != nil {
		stopCh, _ = rawStop.(channels.StopChannel)
	}
	var resyncCh channels.ReSyncChannel
	if rawResync, ok := h.channelPool[channels.ReSync]; ok && rawResync != nil {
		resyncCh, _ = rawResync.(channels.ReSyncChannel)
	}
	return stopCh, resyncCh
}

func (h *Handler) startAndTrackDiscoveryLocked() {
	if h.stopped {
		return
	}
	pipelineCh := make(chan struct{})
	h.currentPipelineCh = pipelineCh
	h.discoveryWg.Add(1)
	go func(ch chan struct{}) {
		defer h.discoveryWg.Done()
		h.Log.Debugf("h.startDiscovery starting with channel %p", ch)
		h.runDiscovery(ch)
		h.Log.Debugf("h.startDiscovery completed for channel %p", ch)
	}(pipelineCh)
}

func (h *Handler) runDiscovery(pipelineCh chan struct{}) {
	if h.DiscoveryStarter != nil {
		h.DiscoveryStarter(pipelineCh)
		return
	}
	h.startDiscovery(pipelineCh)
}

// Only the Run goroutine invokes stopCurrentDiscoveryLocked, ensuring single-owner
// channel closure. The select-default probe is safe from double-close panics.
func (h *Handler) stopCurrentDiscoveryLocked() {
	if h.currentPipelineCh != nil {
		select {
		case <-h.currentPipelineCh:
			// already closed
		default:
			h.Log.Debugf("Closing previous pipelineCh (%p) to stop existing discovery.", h.currentPipelineCh)
			close(h.currentPipelineCh)
		}
	}
}

func (h *Handler) updateInformer() error {
	if h.InformerUpdater != nil {
		return h.InformerUpdater()
	}
	return h.UpdateInformer()
}

// Run drives the discovery lifecycle for this Handler until Stop is closed. It
// must not be invoked concurrently on the same Handler: debounce and discovery
// lifecycle state live on the struct.
func (h *Handler) Run() {
	stopCh, resyncCh := h.getSystemChannels()

	debounceDur := h.DebounceDuration
	if debounceDur <= 0 {
		debounceDur = 5 * time.Second
	}

	h.lifecycleMu.Lock()
	h.stopped = false
	h.startAndTrackDiscoveryLocked()
	h.lifecycleMu.Unlock()

	defer func() {
		h.Log.Debug("Run: defer function executing.")
		h.lifecycleMu.Lock()
		h.stopped = true
		h.stopCurrentDiscoveryLocked()
		h.lifecycleMu.Unlock()

		h.Log.Debug("Run: Waiting for final discovery goroutine to complete...")
		h.discoveryWg.Wait()
		h.Log.Debug("Run: Final discovery goroutine completed.")

		h.ShutdownInformer()
	}()

	var debounceTimer *time.Timer
	var debounceCh <-chan time.Time
	defer func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
	}()

loop:
	for {
		select {
		case <-stopCh:
			h.Log.Debug("Run: Received stop signal. Breaking loop.")
			break loop

		case <-resyncCh:
			h.Log.Debug("Run: Received ReSync signal.")
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(debounceDur)
			debounceCh = debounceTimer.C

		// The debounce tick is handled here on the Run goroutine rather than in a
		// time.AfterFunc callback, so restarts and shutdown share one goroutine
		// and discoveryWg.Add and Wait can never race.
		case <-debounceCh:
			debounceTimer = nil
			debounceCh = nil
			if h.restartDiscovery(stopCh) {
				break loop
			}
		}
	}
	h.Log.Debug("Stopping Run function.")
}

// restartDiscovery drains the previous discovery pipeline and starts a fresh
// one. It reports true when Stop was observed, telling the caller to break its
// loop.
func (h *Handler) restartDiscovery(stopCh channels.StopChannel) bool {
	// Signal previous discovery pipeline to stop
	h.lifecycleMu.Lock()
	if h.stopped {
		h.lifecycleMu.Unlock()
		return true
	}
	h.stopCurrentDiscoveryLocked()
	h.lifecycleMu.Unlock()

	// Wait for existing discovery goroutine to finish
	h.Log.Debug("Waiting for existing discovery goroutine to complete...")
	h.discoveryWg.Wait()
	h.Log.Debug("Existing discovery goroutine completed.")

	// Stop may have arrived while the previous pipeline was draining; honor it
	// before starting anything new.
	if stopCh != nil {
		select {
		case <-stopCh:
			h.Log.Debug("Run: Received stop signal while waiting for discovery. Breaking loop.")
			return true
		default:
		}
	}

	// Update informer before starting new discovery
	if err := h.updateInformer(); err != nil {
		h.Log.Error(fmt.Errorf("failed to update informer: %w", err))
	}

	h.lifecycleMu.Lock()
	if h.stopped {
		h.lifecycleMu.Unlock()
		return true
	}
	h.Log.Debug("Starting new discovery process...")
	h.startAndTrackDiscoveryLocked()
	h.lifecycleMu.Unlock()
	return false
}

func (h *Handler) UpdateInformer() error {
	if h.kubeClient == nil {
		return fmt.Errorf("failed to update informer: kubeClient is nil")
	}
	dynamicClient, err := dynamic.NewForConfig(&h.kubeClient.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create new dynamic client for informer: %w", err)
	}
	listOptionsFunc, err := GetListOptionsFunc(h.Config)
	if err != nil {
		return fmt.Errorf("failed to get list options func for informer: %w", err)
	}
	h.lifecycleMu.Lock()
	defer h.lifecycleMu.Unlock()
	if h.informer != nil {
		h.Log.Debug("Shutting down existing informer before update.")
		h.informer.Shutdown()
	}
	h.Log.Debug("Creating new dynamic shared informer factory.")
	h.informer = GetDynamicInformer(h.Config, dynamicClient, listOptionsFunc)
	return nil
}

func (h *Handler) ShutdownInformer() {
	h.lifecycleMu.Lock()
	defer h.lifecycleMu.Unlock()
	if h.informer != nil {
		h.Log.Debug("Shutting down informer...")
		h.informer.Shutdown()
		// Informer is nulled after shutdown so that the deferred ShutdownInformer call
		// in pkg/lib/meshsync/meshsync.go remains an idempotent safe no-op.
		h.informer = nil
		h.Log.Debug("Shutting down informer done.")
	}
}

func (h *Handler) ListenToRequests() {
	listenerConfigs, err := h.loadListenerConfigs()
	if err != nil {
		h.Log.Error(err)
	}
	reqChan, err := h.initRequestListener(listenerConfigs)
	if err != nil {
		h.Log.Error(ErrSubscribeRequest(err))
	}

loop:
	for {
		select {
		case <-h.channelPool[channels.Stop].(channels.StopChannel):
			break loop
		case request := <-reqChan:
			h.handleRequest(request, listenerConfigs)
		}
	}
	h.Log.Debug("Stopping ListenToRequests")
}

func (h *Handler) loadListenerConfigs() (map[string]config.ListenerConfig, error) {
	listenerConfigs := make(map[string]config.ListenerConfig, 10)
	if err := h.Config.GetObject(config.ListenersKey, &listenerConfigs); err != nil {
		return nil, ErrGetObject(err)
	}
	return listenerConfigs, nil
}

func (h *Handler) initRequestListener(
	listenerConfigs map[string]config.ListenerConfig,
) (chan *broker.Message, error) {

	reqChan := make(chan *broker.Message)
	cfg := listenerConfigs[config.RequestStream]

	h.Log.Debugf("Listening for requests in: %s", cfg.SubscribeTo)
	return reqChan, h.Broker.SubscribeWithChannel(
		cfg.SubscribeTo,
		cfg.ConnectionName,
		reqChan,
	)
}

func (h *Handler) handleRequest(
	request *broker.Message,
	listenerConfigs map[string]config.ListenerConfig,
) {
	if request.Request == nil {
		h.Log.Error(ErrInvalidRequest)
		return
	}

	switch request.Request.Entity {
	case broker.LogRequestEntity:
		h.handleLogRequest(request, listenerConfigs)

	case "informer-store":
		h.handleInformerStoreRequest(request)

	case broker.ReSyncDiscoveryEntity:
		h.handleResyncRequest()

	case broker.ExecRequestEntity:
		h.handleExecRequest(request, listenerConfigs)

	case broker.ActiveExecEntity:
		h.handleActiveExecRequest()

	case "meshsync-meta":
		h.publishMeshSyncMeta()
	}
}

func (h *Handler) handleLogRequest(
	request *broker.Message,
	listenerConfigs map[string]config.ListenerConfig,
) {
	h.Log.Debug("Starting log session")
	if err := h.processLogRequest(
		request.Request.Payload,
		listenerConfigs[config.LogStream],
	); err != nil {
		h.Log.Error(err)
		return
	}
}

// TODO: Add this to the broker pkg
func (h *Handler) handleInformerStoreRequest(request *broker.Message) {
	// TODO: Update broker pkg in Meshkit to include Reply types
	var payload struct{ Reply string }

	data, err := json.Marshal(request.Request.Payload)
	if err != nil {
		h.Log.Error(err)
		return
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		h.Log.Error(err)
		return
	}

	objects := splitIntoMultipleSlices(h.listStoreObjects(), 5) //  performance of NATS is bound to degrade if huge messages are sent
	h.Log.Debugf("Publishing the data from informer stores to the subject: %s", payload.Reply)

	for _, batch := range objects {
		if err := h.Broker.Publish(payload.Reply, &broker.Message{Object: batch}); err != nil {
			h.Log.Error(err)
		}
	}
}

func (h *Handler) handleResyncRequest() {
	h.Log.Debug("Resyncing")
	stopCh, resyncCh := h.getSystemChannels()
	if resyncCh != nil {
		if !resyncCh.ReSyncInformerWithStop(stopCh) {
			h.Log.Debug("Dropping resync request: stop signal received")
		}
	}
}

func (h *Handler) handleExecRequest(
	request *broker.Message,
	listenerConfigs map[string]config.ListenerConfig,
) {
	h.Log.Debug("Starting interactive session")
	if err := h.processExecRequest(
		request.Request.Payload,
		listenerConfigs[config.ExecShell],
	); err != nil {
		h.Log.Error(err)
		return
	}
}

func (h *Handler) handleActiveExecRequest() {
	h.Log.Debug("Connecting to channel pool")
	if err := h.processActiveExecRequest(); err != nil {
		h.Log.Error(err)
		return
	}
}

func (h *Handler) publishMeshSyncMeta() {
	h.Log.Debug("Publishing MeshSync metadata to the subject")
	err := h.Broker.Publish("meshsync-meta", &broker.Message{
		Object: config.Server["version"],
	})
	if err != nil {
		h.Log.Error(err)
		return
	}
}

func (h *Handler) listStoreObjects() []model.KubernetesResource {
	objects := make([]interface{}, 0)
	for _, v := range h.stores {
		objects = append(objects, v.List()...)
	}
	parsedObjects := make([]model.KubernetesResource, 0)
	for _, obj := range objects {
		parsedObjects = append(
			parsedObjects,
			model.ParseList(
				*obj.(*unstructured.Unstructured),
				broker.Add,
				h.clusterID,
			),
		)
	}
	return parsedObjects
}

const (
	// watchCRDsBaseDelay is the pause between CRD watch iterations under normal
	// conditions - the API server routinely closes long-lived watches and we
	// simply re-establish them.
	watchCRDsBaseDelay = 2 * time.Second
	// watchCRDsMaxDelay caps the exponential backoff applied to failing or
	// immediately-collapsing watches so a persistently unreachable API server is
	// retried at a steady, bounded rate instead of in a tight loop.
	watchCRDsMaxDelay = 2 * time.Minute
)

func (h *Handler) WatchCRDs() {
	h.Log.Debug("meshsync::Handler::WatchCRDs: starting WatchCRDs")

	backoff := watchCRDsBaseDelay
loop:
	for {
		// watchCRDsIteration returns nil when the stop channel is closed or the
		// API server closes a healthy watch; it returns an error when the watch
		// could not be established.
		start := time.Now()
		err := h.watchCRDsIteration()
		lasted := time.Since(start)

		var delay time.Duration
		// Treat an iteration as healthy only if it ran without error for at least
		// the base delay. A watch that errors, or that collapses almost
		// immediately (e.g. RBAC denial, API server churn), is backed off
		// exponentially with jitter; a watch that stayed up and cycled normally
		// resets the backoff so routine re-establishment is not penalised.
		if err == nil && lasted >= watchCRDsBaseDelay {
			backoff = watchCRDsBaseDelay
			delay = watchCRDsBaseDelay
		} else {
			if err != nil {
				h.Log.Warnf("meshsync::Handler::WatchCRDs: watch iteration failed, backing off %s: %v", backoff, err)
			} else {
				h.Log.Debugf("meshsync::Handler::WatchCRDs: watch collapsed after %s, backing off %s", lasted, backoff)
			}
			delay = jitter(backoff)
			backoff = min(backoff*2, watchCRDsMaxDelay)
		}

		// Wait before the next iteration, but wake immediately on a stop signal
		// to ensure a fast shutdown.
		select {
		case <-h.channelPool[channels.Stop].(channels.StopChannel):
			break loop
		case <-time.After(delay):
		}
	}
	h.Log.Debug("meshsync::Handler::WatchCRDs: stopping WatchCRDs")
}

// jitter returns a duration in [d/2, d] to spread out watch re-establishment
// retries and avoid many MeshSync instances retrying in lockstep against the
// API server.
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

func (h *Handler) watchCRDsIteration() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	crdWatcher, err := h.createCRDWatcher(ctx)
	if err != nil {
		return err
	}
	// Stop the watcher explicitly rather than relying on ctx cancellation alone,
	// so the streaming connection and its goroutine are released immediately when
	// this iteration returns (stop signal, closed channel, or re-establishment).
	defer crdWatcher.Stop()

	for {
		select {
		case <-h.channelPool[channels.Stop].(channels.StopChannel):
			return nil
		case event, ok := <-crdWatcher.ResultChan():
			if !ok {
				h.Log.Debug("meshsync::Handler::watchCRDsIteration crdWatcher.ResultChan() was closed")
				return nil
			}
			h.handleCRDEvent(event)
		}
	}
}

func (h *Handler) createCRDWatcher(
	ctx context.Context,
) (watch.Interface, error) {
	return h.kubeClient.DynamicKubeClient.Resource(
		schema.GroupVersionResource{
			Group:    "apiextensions.k8s.io",
			Version:  "v1",
			Resource: "customresourcedefinitions",
		},
	).Watch(ctx, metav1.ListOptions{})
}

func (h *Handler) handleCRDEvent(event watch.Event) {
	if event.Object == nil {
		h.Log.Debug("Handler::WatchCRDs::processEvent event.Object is nil, skipping")
		return
	}

	crd, data, err := parseCRDEvent(event)
	if err != nil {
		h.Log.Error(err)
		return
	}

	if len(crd.Spec.Versions) == 0 {
		h.Log.Debugf(
			"Handler::WatchCRDs::processEvent: event.Object has empty spec.Versions [%s]",
			string(data),
		)
	}

	gvr := tmpGetGVRForCustomResources(crd)
	if gvr == nil {
		return
	}

	changed, err := h.updatePipelineConfig(event.Type, gvr)
	if err != nil {
		h.Log.Error(err)
		h.Log.Debug("skipping informer resync")
		return
	}

	if !changed {
		// The set of watched resources is unchanged (e.g. a MODIFIED event), so a
		// resync would tear down and rebuild every informer - re-listing all
		// watched resource types cluster-wide - for no benefit. Skip it.
		h.Log.Debugf("crd %s event for %s did not change discovery config; skipping informer resync", event.Type, gvr.String())
		return
	}

	h.Log.Debug("Resyncing informer from watch crd")
	stopCh, resyncCh := h.getSystemChannels()
	if resyncCh != nil {
		if !resyncCh.ReSyncInformerWithStop(stopCh) {
			h.Log.Debug("Dropping CRD resync: stop signal received")
		}
	}
}

func parseCRDEvent(event watch.Event) (*kubernetes.CRDItem, []byte, error) {
	data, err := json.Marshal(event.Object)
	if err != nil {
		return nil, nil, err
	}

	crd := &kubernetes.CRDItem{}
	if err := json.Unmarshal(data, crd); err != nil {
		return nil, nil, err
	}

	return crd, data, nil
}

// updatePipelineConfig reflects a CRD watch event in the discovery pipeline
// config. It returns changed=true only when the set of watched resources was
// actually mutated (a resource registered or removed), so callers can skip the
// expensive informer resync for events that leave discovery untouched - notably
// MODIFIED events, which controllers such as cert-manager's cainjector emit
// continuously as they rewrite unrelated CRD fields (e.g. the conversion webhook
// caBundle). The GVR is derived solely from group/version/plural, none of which
// a MODIFIED event alters, so such events never change what we watch.
func (h *Handler) updatePipelineConfig(
	eventType watch.EventType,
	gvr *schema.GroupVersionResource,
) (changed bool, err error) {

	existing := make(map[string]config.PipelineConfigs)
	if err := h.Config.GetObject(config.ResourcesKey, &existing); err != nil {
		return false, err
	}

	pipelines := existing[config.GlobalResourceKey]
	name := fmt.Sprintf("%s.%s.%s", gvr.Resource, gvr.Version, gvr.Group)

	switch eventType {
	case watch.Added:
		// Adding is idempotent. A freshly (re)established CRD watch re-lists every
		// existing CRD as ADDED, so registering unconditionally would accumulate
		// duplicate pipeline configs - which in turn register duplicate informer
		// event handlers and double-publish every resource. Only register (and
		// resync for) resources we are not already watching.
		if pipelines.Contains(name) {
			return false, nil
		}
		pipelines = pipelines.Add(config.PipelineConfig{
			Name:      name,
			PublishTo: config.DefaultPublishingSubject,
			// Dynamically discovered CRD pipelines watch the canonical default
			// event set; keep them sourced from one place to avoid drift.
			Events: config.DefaultEvents,
		})
	case watch.Deleted:
		if !pipelines.Contains(name) {
			return false, nil
		}
		pipelines = pipelines.Delete(config.PipelineConfig{Name: name})
	default:
		// MODIFIED, BOOKMARK, ERROR: the watched-resource set is unchanged, so
		// there is nothing to persist and no reason to resync.
		return false, nil
	}

	existing[config.GlobalResourceKey] = pipelines
	if err := h.Config.SetObject(config.ResourcesKey, existing); err != nil {
		return false, err
	}
	return true, nil
}

// TODO: move this to meshkit
// given [1,2,3,4,5,6,7,5,4,4] and 3 as its arguments, it would
// return [[1,2,3], [4,5,6], [7,5,4], [4]]
func splitIntoMultipleSlices(s []model.KubernetesResource, maxItmsPerSlice int) []([]model.KubernetesResource) {
	result := make([]([]model.KubernetesResource), 0)
	temp := make([]model.KubernetesResource, 0)

	for idx, val := range s {
		temp = append(temp, val)
		if ((idx + 1) % maxItmsPerSlice) == 0 {
			result = append(result, temp)
			temp = nil
		}
		if idx+1 == len(s) {
			if len(temp) != 0 {
				result = append(result, temp)
			}
		}
	}

	return result
}

// TODO: this is temp fix, original is here
// https://github.com/meshery/meshkit/blob/master/utils/kubernetes/crd.go#L49C6-L49C30
// it is panics if crd.Spec.Versions is empty
func tmpGetGVRForCustomResources(crd *kubernetes.CRDItem) *schema.GroupVersionResource {
	if crd == nil {
		return nil
	}

	if len(crd.Spec.Versions) > 0 {
		return kubernetes.GetGVRForCustomResources(crd)
	}

	return &schema.GroupVersionResource{
		Group:    crd.Spec.Group,
		Version:  "",
		Resource: crd.Spec.Names.ResourceName,
	}
}
