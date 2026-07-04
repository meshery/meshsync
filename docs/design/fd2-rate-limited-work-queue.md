I have full grounding now. Let me construct the blueprint.

---

# Implementation Blueprint: Rate-Limited Work Queue + Converter (Backpressure)

## 1. Goal & Gap

**Goal**: Insert a bounded, rate-limited FIFO between informer event delivery and output-writer publish, so a slow/unavailable NATS broker cannot stall the shared informer's event-processing goroutine, and so mass-rollout bursts are smoothed rather than hammering the broker 1:1 with API-server events.

**Gap** (confirmed by reading, not assumed): `internal/pipeline/handlers.go:17-74` calls `ri.publishItem` synchronously inside `AddFunc`/`UpdateFunc`/`DeleteFunc`, which calls `ri.outputWriter.Write(...)` at `handlers.go:102` and blocks on its return. `output.BrokerWriter.Write` (`internal/output/broker.go:19-32`) is a direct, synchronous call into `broker.Handler.Publish`, whose NATS implementation (`meshkit/broker/nats/nats.go:143-167`) calls `nc.Publish()` - itself synchronous from the caller's point of view. `client-go`'s shared informer delivers all events for a given `GroupVersionResource` through **one single-threaded processing goroutine** (the informer's `sharedProcessor` fans out to listeners serially); blocking that goroutine on a slow broker call stalls delivery of every subsequent event for that resource kind, and errors are logged and dropped (`ri.log.Error(err)` at handlers.go:21/36/71) with no retry, no queue, and no metric. There is currently zero buffering between "informer decided this changed" and "write blocked on I/O."

## 2. Current-State Per Repo (cited)

**MeshSync** (`meshsync`):
- `internal/pipeline/handlers.go:15-76` - `GetEventHandlers()` returns `cache.ResourceEventHandlerFuncs` whose three funcs each synchronously call `publishItem` inline, no goroutine, no channel.
- `internal/pipeline/handlers.go:25-46` - `UpdateFunc` already does the **only** existing suppression: parses `resourceVersion` on old/new objects and skips the publish if `oldRV >= newRV` (a no-op resync re-delivery). This is orthogonal to queue-level coalescing - it fires before anything reaches a queue.
- `internal/pipeline/handlers.go:82-112` - `publishItem` converts via `model.ParseList`, checks `checkMustSkip` (namespace filtration), then calls `ri.outputWriter.Write` and propagates/logs its error. This is the exact synchronous hand-off point that must become an enqueue.
- `internal/pipeline/pipeline.go:15-77` (`New`) - constructs fresh `RegisterInformer` steps (`internal/pipeline/step.go:25-41`) on **every** discovery/resync call (per this repo's `CLAUDE.md` rule 4 and `docs/agent-instructions/architecture.md`); `RegisterInformer.Exec` (`step.go:45-68`) is what calls `ri.registerHandlers(iclient.Informer())`, i.e. `AddEventHandler`. Any worker-pool/queue lifetime tied to a discovery run must be created and torn down inside this same per-run construction, not hoisted to package scope.
- `internal/output/output.go:9-15` - the `Writer` interface (`Write(obj, evtype, config) error`) is the single seam every existing writer (`BrokerWriter`, `FileWriter`, `CompositeWriter`, both dedup decorators) implements. A queue-backed writer that satisfies this same interface is the natural insertion point with zero call-site changes beyond wiring.
- `internal/output/inmemory_deduplicator_streaming.go:11-49` - `InMemoryDeduplicatorStreamingWriter` dedups by **UID presence** (first-seen-wins, never republishes the same UID again for the process lifetime) - this is **not** content-hash dedup and is wired only in file-output mode (`pkg/lib/meshsync/meshsync.go:189-193`), inside a `CompositeWriter` alongside an unfiltered "extended" file writer. It is **not** in the broker path at all today - `pkg/lib/meshsync/meshsync.go:135-139` wires `BrokerWriter` directly with no dedup decorator.
- `internal/output/inmemory_deduplicator.go:15-99` - the batch/flush-on-exit variant, likewise unused on the broker path.
- `meshkit/broker/nats/nats.go:143-167` (`Publish`) - synchronous `nc.Publish()`; the underlying `nats.go` client has its own internal buffer, but under sustained broker unavailability (`MaxReconnect` exhaustion, server-side slow-consumer) `Publish` will error, and `handlers.go` just logs and drops - no retry, no backoff, no requeue.
- `meshsync/handlers.go:41-113` (`Run`) - the resync loop that calls `h.startDiscovery` (which calls `pipeline.New` -> `pl.Run()`) fresh on every `ReSync` signal, confirming the per-run construction constraint applies transitively from `Handler.Run` down through `pipeline.New` to `RegisterInformer`.
- `k8s.io/client-go v0.35.3` is already a direct dependency (`go.mod:20`) - `k8s.io/client-go/util/workqueue` is available **today with zero new external dependencies**. At this version the package's non-generic `Interface`/`RateLimitingInterface`/`DelayingInterface` are deprecated aliases for `TypedInterface[any]`/`TypedRateLimitingInterface[any]`/`TypedDelayingInterface[any]`; the current API is generic (`TypedRateLimitingQueue[T comparable]`, `NewTypedRateLimitingQueueWithConfig[T]`, `DefaultTypedControllerRateLimiter[T]`).
- `internal/config/types.go:50-54` (`PipelineConfig`) has only `Name`, `PublishTo`, `Events` - no worker-count/rate-limit knobs anywhere in `internal/config`.
- `helpers/component_info.json:4` - `next_error_code: 1015`, but `internal/pipeline/error.go:7-13` already allocates `1015` (`ErrWriteOutputCode`) - the true next-available code for this feature's errors is **1016** (the `error-codes-updater` workflow will reconcile the counter on merge; per `docs/agent-instructions/errors.md`, allocate a placeholder and let CI normalize it).

**Meshery Operator** (`meshery-operator`):
- `api/v1alpha1/meshsync_types.go:38-46` and `api/v1alpha2/meshsync_types.go:38-46` - `MeshSyncSpec` has `WatchList corev1.ConfigMap`, `Broker MeshsyncBroker`, `Version string`, `Size int32` (replica count, 1-10). **No queue/worker/rate-limit field exists in either CRD version today.**

**Meshery Server** (`meshery/server`) and **MeshKit** (`meshkit`): consume the NATS wire contract (`meshery.meshsync.core` subject, `pkg/model.KubernetesResource` payload) unchanged by this feature - the queue is purely an internal MeshSync scheduling mechanism; it does not alter the published message shape, ordering-per-key semantics aside (see 8).

## 3. Proposed Architecture

### Queue choice: `k8s.io/client-go/util/workqueue.TypedRateLimitingInterface[string]`, not a custom channel

Use the **standard client-go controller pattern** (informers enqueue keys; workers dequeue, re-fetch/convert, process) rather than a hand-rolled bounded channel, for these reasons grounded in what was read above:

1. **It already exists as an unused, zero-cost dependency.** `client-go` is pinned at `v0.35.3` and `dynamicinformer`/`cache` are already imported throughout `internal/pipeline` and `meshsync`. Adding `util/workqueue` adds no new module.
2. **It gives coalescing-by-key for free** via `Add(item T)` on an already-queued key being a no-op until `Done` - which is exactly the "dedup rapid successive updates to the same object while queued" requirement, without hand-writing a dedup map (a custom channel does not have this; you would rebuild `workqueue.Typed`'s guts).
3. **It gives rate limiting for free** via `TypedRateLimitingInterface[T].AddRateLimited`/`NumRequeues`/`Forget`, with a proven `DefaultTypedControllerRateLimiter[T]()` (exponential-backoff-per-item + a global token bucket) - this is precisely "FIFO queue capable of being rate-limited" from the design goal, using a component the client-go ecosystem has hardened for exactly this failure mode (retry a Kubernetes-adjacent I/O call without hot-looping).
4. **`ShutDownWithDrain()` gives graceful shutdown/drain for free** - workers finish in-flight items, no new adds are accepted, `ShutDown` unblocks all `Get()` callers.
5. A custom bounded channel would need to reimplement: per-key coalescing (a `map[key]struct{}` "in queue" set), rate limiting (a token bucket or backoff timer), and drain-on-shutdown semantics - all already correct and tested in `workqueue`. The one thing a channel is simpler for - strict global FIFO of every raw event - is explicitly **not** the requirement here; per-key coalescing is.

### Enqueue-key vs enqueue-object semantics: **enqueue-key**

Enqueue a **string key** (`cache.MetaNamespaceKeyFunc`-style: `namespace/name` plus the resource's GVR, since MeshSync watches many kinds concurrently unlike a single-resource operator), not the raw `*unstructured.Unstructured` object, with one exception carved out below for DELETE.

Rationale:
- This is the textbook client-go workqueue pattern for a reason: re-fetching current state from the informer's local store (`cache.Store`, already retained per-resource at `step.go:59-63`/`data[ri.config.Name] = iclient.Informer().GetStore()`) at dequeue time means a worker converting/publishing key `X` always converts the **latest** object, not a stale snapshot captured at enqueue time. If three updates to the same object race into the queue, `workqueue`'s de-dup collapses them to one queued key, and the worker fetches whatever is current in the store when it finally dequeues - correctly reflecting "coalesce rapid successive updates," not "process update #1 then #2 then #3."
- Enqueuing full objects instead would defeat the coalescing benefit (three different object pointers can't collapse into one queue slot the way three identical string keys can) and would hold three copies of a potentially large object in memory during a burst.

**DELETE exception**: a delete removes the object from the informer's store **before** or **concurrently with** the event firing, so a worker cannot "re-fetch current state" for a delete - there is no current state. Carry the terminal object alongside the key for deletes (see Component Design below, `queuedEvent.deleteObj`), captured at enqueue time from the tombstone-unwrapped object already produced by the existing `DeleteFunc` (`handlers.go:48-74`).

### Worker pool of converters

A fixed-size pool of goroutines (`Get()` -> convert -> `outputWriter.Write` -> `Done()`/`Forget()`/`AddRateLimited()`), started once per `pipeline.New` call (per-run, matching the existing per-run stage-construction constraint) and shut down when that run's `stopChan` closes.

### Data flow

```
informer event (Add/Update/Delete)
        |
        v
handlers.go AddFunc/UpdateFunc/DeleteFunc  (unchanged: RV-suppression, tombstone unwrap, checkMustSkip stay HERE - cheap, in-process checks that should not even reach the queue)
        |
        v
QueuingWriter.Write(obj, evtype, config)      <-- NEW: satisfies output.Writer, becomes ri.outputWriter
        |  builds queuedKey{gvr, namespace, name}; for Delete, also stashes obj+evtype+config in a side map keyed by queuedKey
        v
workqueue.TypedRateLimitingInterface[queuedKey]   <-- bounded via QueueConfig, in-queue dedup free
        |
        v (N worker goroutines, N = WorkerCount)
worker: key, shutdown := queue.Get()
        |  if shutdown: return
        |  look up latest object: informer store (Add/Update) OR side-map stash (Delete)
        |  re-run model.ParseList + checkMustSkip equivalent (already done before enqueue; re-check store presence for Add/Update racing a Delete)
        v
realWriter.Write(k8sResource, evtype, config)   <-- the EXISTING output.Writer chain: BrokerWriter | CompositeWriter | dedup decorators, unmodified
        |
        on success: queue.Forget(key); queue.Done(key)
        on error:   queue.AddRateLimited(key); queue.Done(key)   <-- retry via workqueue's own backoff, not a busy loop
```

Overflow policy sits at the `Add`/enqueue call inside `QueuingWriter.Write` (detailed in 8).

## 4. Per-Repo Changes

### MeshSync (all work lives here; this is "mostly MeshSync-internal" per the task framing)

**Create `internal/output/queue.go`** - the core new component:
- `type QueuingWriter struct` wrapping: the real downstream `output.Writer`, a `workqueue.TypedRateLimitingInterface[queuedKey]`, a `logger.Handler`, a bounded/overflow policy, an informer-store lookup function (`func(gvr string) (cache.Store, bool)`, injected so this package does not import `internal/pipeline` and create a cycle), a `sync.Map` (or mutex+map) side-stash for DELETE payloads keyed by `queuedKey`, and observability hooks (counters, described in 4's metrics file).
- `type queuedKey struct { gvr, namespace, name string }` - `comparable`, satisfies `workqueue`'s type constraint.
- `func NewQueuingWriter(real output.Writer, storeLookup func(string) (cache.Store, bool), log logger.Handler, cfg QueueConfig) *QueuingWriter` - constructs the `workqueue.TypedRateLimitingQueueConfig[queuedKey]` (with a `MetricsProvider` wired to the new metrics hooks - see below), builds the queue via `workqueue.NewTypedRateLimitingQueueWithConfig`, and **starts `cfg.WorkerCount` worker goroutines immediately** (constructor eagerly starts workers - matches this repo's existing pattern of eager-start-in-constructor, e.g. `healthServer.start` returning a stop func).
- `func (w *QueuingWriter) Write(obj model.KubernetesResource, evtype broker.EventType, config internalconfig.PipelineConfig) error` - satisfies `output.Writer`. For `Delete`, stashes `{obj, evtype, config}` in the side map first (so it's available even if the worker races ahead of this function returning), then calls the bounded-overflow-aware enqueue helper. Returns immediately (non-blocking on the happy path) - this is the load-bearing change that decouples the informer goroutine from broker latency.
- `func (w *QueuingWriter) Shutdown(drain bool)` - calls `queue.ShutDownWithDrain()` if `drain` else `queue.ShutDown()`, used from the per-run teardown path described below.
- `runWorker()` - the dequeue loop: `key, shutdown := w.queue.Get()`; on shutdown return; resolve current object (store lookup for Add/Update, side-map pop for Delete); if Add/Update and the key is no longer in the store (deleted between enqueue and dequeue), treat as a no-op (the eventual DELETE event will have its own queued entry - never synthesize a delete here, that's exactly the kind of silent behavior change this repo's `checkMustSkip`-adjacent logic must avoid); convert via the already-existing `model.ParseList` equivalent (the object handed to `Write` is already a `model.KubernetesResource` by the time it reaches `QueuingWriter` - see note below on where conversion actually happens - so "convert" here is a re-fetch-and-reconvert only for the coalesced Add/Update case, not a new conversion step); call `w.real.Write(...)`; on error call `w.queue.AddRateLimited(key)` and log at Warn (not Error, since retry is expected and will resolve or exhaust); on success call `w.queue.Forget(key)`; always call `w.queue.Done(key)`.

**Important design refinement found during grounding**: `model.ParseList` conversion (`handlers.go:93`) happens **before** `outputWriter.Write` is called, inside `publishItem`, not inside the writer. This means the `model.KubernetesResource` handed to `QueuingWriter.Write` is already converted - it is a **converted-object cache**, not a raw `unstructured.Unstructured`. Two consistent options, and the one that matches "enqueue-key" fidelity is chosen:

- **Chosen**: `QueuingWriter` stores the **latest converted `model.KubernetesResource` per key** in its own map (updated on every `Write` call, overwriting the prior value for that key), rather than doing a second store lookup + reconversion at dequeue time. This avoids threading informer `cache.Store` access and GVR-to-config plumbing into `internal/output` (which today has zero dependency on `internal/pipeline` or `dynamicinformer` - preserving that layering is worth the small memory cost of one `model.KubernetesResource` per distinct in-flight key, bounded by queue capacity). `Write` always updates `w.latest[key] = {obj, evtype, config}` **then** enqueues the key (idempotent add if already queued) - this is the coalescing point: three updates to the same key overwrite `w.latest[key]` twice before the worker ever dequeues once, and the worker publishes only the final value. This removes the need for the informer-store-lookup function entirely, simplifying the constructor signature (drop `storeLookup` from the design above) and removing any coupling to `internal/pipeline`.
- This makes the DELETE side-stash and the Add/Update "latest" map the **same map** (`map[queuedKey]latestEvent`), simplifying the implementation to one path instead of two.

Revised `QueuingWriter.Write`:
```go
func (w *QueuingWriter) Write(obj model.KubernetesResource, evtype broker.EventType, config internalconfig.PipelineConfig) error {
    key := queuedKeyFor(obj) // namespace/name/kind - derived from obj.KubernetesResourceMeta + obj.Kind
    w.mu.Lock()
    w.latest[key] = latestEvent{obj: obj, evtype: evtype, config: config}
    w.mu.Unlock()
    return w.enqueue(key) // handles overflow policy; Add() is idempotent if key already queued
}
```
Worker:
```go
func (w *QueuingWriter) runWorker() {
    for {
        key, shutdown := w.queue.Get()
        if shutdown { return }
        w.mu.Lock()
        ev, ok := w.latest[key]
        delete(w.latest, key) // claim it; a Write racing in after this point re-adds under the same key, which is correct - it's a new event
        w.mu.Unlock()
        if !ok {
            w.queue.Done(key)
            continue // coalesced away, nothing to do (should not normally happen given the lock ordering, but must not panic if it does)
        }
        if err := w.real.Write(ev.obj, ev.evtype, ev.config); err != nil {
            w.log.Warnf("meshsync: queued write failed for %v, will retry: %v", key, err)
            w.metrics.recordRetry(key.kind)
            w.queue.AddRateLimited(key)
        } else {
            w.queue.Forget(key)
            w.metrics.recordProcessed(key.kind, evtype)
        }
        w.queue.Done(key)
    }
}
```
This is the concrete, load-bearing design decision: **coalescing is a "latest-value map keyed by object identity" pattern, with the workqueue used purely as the trigger/scheduling/backoff mechanism, not as the value carrier.** This is a well-established variant of the client-go workqueue pattern (it's literally how most real controllers use it - the queue carries keys, a separate informer-store or cache carries values) adapted here to use MeshSync's own converted-object cache instead of `client-go`'s `cache.Store` (which holds raw API objects, not `model.KubernetesResource`).

**Modify `internal/output/output.go`**: no interface change needed - `QueuingWriter` satisfies `Writer` as-is. Optionally add a narrow `ShutdownableWriter` interface (`Shutdown(drain bool)`) so `pkg/lib/meshsync/meshsync.go` and `internal/pipeline` can type-assert for graceful drain without a hard dependency from `output.Writer`'s core contract - keeps existing writers (`FileWriter`, `BrokerWriter`) untouched, since not every writer needs a shutdown hook.

**Modify `internal/pipeline/pipeline.go`**: `New(...)` currently takes an `ow output.Writer` parameter and threads it unchanged to every `newRegisterInformerStep` call (`pipeline.go:40,55`). Wrap the caller-supplied writer in a `QueuingWriter` **inside `New`**, once per call (matching the mandatory per-run-construction rule), before passing it to the steps:
```go
queuingWriter := output.NewQueuingWriter(ow, log, output.QueueConfig{
    WorkerCount: cfg.WorkerCount, // from PipelineConfig or a new top-level config key, see section 5
    ...
})
// register a shutdown hook so StartInformers' stopChan closing drains the queue - see step.go change below
```
Pass `queuingWriter` (not the raw `ow`) into `newRegisterInformerStep(...)`. This confines all queue lifetime management to the pipeline construction/teardown boundary that this repo's architecture doc already establishes as the correct seam for per-run resources.

**Modify `internal/pipeline/step.go`**: `StartInformers.Exec` (`step.go:100-122`) already blocks on `WaitForCacheSync(stopChan)` and is the step that owns the run's `stopChan`. Add a small goroutine or defer here (or, cleaner, in a new `Cancel()` path on a stage wrapper) that calls `queuingWriter.Shutdown(drain: true)` when `stopChan` closes, so:
- Normal resync: queue drains in-flight items (bounded by drain, not indefinite - pair with a drain timeout, see 8) before the next `pipeline.New` call constructs a fresh queue.
- Process shutdown (`meshsyncHandler.ShutdownInformer` -> `chPool[channels.Stop]` closing, per `meshsync/handlers.go:134-140`): same drain path, bounded by the overall process shutdown grace period.

**Modify `internal/pipeline/handlers.go`**: **no functional change** to `AddFunc`/`UpdateFunc`/`DeleteFunc`/`publishItem` bodies - they keep calling `ri.outputWriter.Write(...)` exactly as today; `ri.outputWriter` is now a `*output.QueuingWriter` instead of the raw `BrokerWriter`/`CompositeWriter`, transparently, because of the `pipeline.go` wrapping above. This is the payoff of choosing "wrap at the `output.Writer` seam": the handler code, which already has the RV-suppression and tombstone-unwrap logic this repo has hardened, is untouched.

**Create `internal/output/queue_error.go`** (or extend the existing `internal/pipeline/error.go` - pick `internal/output` since the queue lives there): new MeshKit structured errors for: enqueue failure under a "block" overflow policy that times out, drain-timeout-exceeded during shutdown. Allocate placeholder codes starting at `1016`+ per `docs/agent-instructions/errors.md` (do not hand-number past what CI will reconcile).

**Create `internal/output/metrics.go`**: a small `MetricsProvider` implementation satisfying `workqueue.MetricsProvider` (the interface `workqueue.NewTypedRateLimitingQueueWithConfig`'s `MetricsProvider` field expects), recording: queue depth (`NewDepthMetric`), adds (`NewAddsMetric`), latency (`NewLatencyMetric`), work-duration (`NewWorkDurationMetric`), retries (`NewRetriesMetric`), plus a MeshSync-specific overflow-drop counter not covered by `workqueue.MetricsProvider`. This module intentionally exposes plain counters/gauges behind a small interface (e.g. `type Metrics interface { SetDepth(int); IncAdds(); ObserveLatency(time.Duration); IncRetries(); IncDrops(reason string) }`) rather than importing a Prometheus client library directly, so it composes cleanly with the **planned `/metrics` feature** (task's explicit pairing) without this feature needing to decide the eventual exposition format. The `/metrics` feature, when built, implements `Metrics` with a real `prometheus.Registry`-backed type; until then, a no-op or simple in-memory implementation satisfies it so the queue feature ships independently.

**Modify `pkg/lib/meshsync/meshsync.go`**: no change required to the writer-construction block (`lines 115-201`) - the wrapping happens one layer down inside `pipeline.New`, which already receives `outputProcessor` (itself wrapping the real writer) as `ow` via `meshsync.New(...) -> h.outputWriter -> startDiscovery -> pipeline.New(..., h.outputWriter, ...)`. This is a meaningful confirmation that **no plumbing change is needed above the pipeline layer** - the queue is entirely internal to `internal/pipeline` + `internal/output`, matching the task's "mostly MeshSync-internal" framing precisely.

**Modify `internal/config/types.go`**: add worker-count/rate-limit fields (see section 5).

**Test files** (see section 9 for content): `internal/output/queue_test.go` (new), update `internal/pipeline/pipeline_test.go` and `internal/pipeline/handlers_test.go`'s `recordingWriter` usage if the wrapping changes what `ri.outputWriter` is in tests (tests construct `RegisterInformer` directly with a `recordingWriter`, bypassing `pipeline.New`, so they remain valid as direct-writer tests of handler logic; add new pipeline-level tests that go through `pipeline.New` to assert the queue wrapping happens and drains correctly).

### Meshery Operator (`meshery-operator`)

**Modify `api/v1alpha1/meshsync_types.go` and `api/v1alpha2/meshsync_types.go`**: add an optional `Backpressure` sub-struct to `MeshSyncSpec`, mirroring the existing `MeshsyncBroker` sub-struct pattern already in this file:
```go
type MeshsyncBackpressure struct {
    // WorkerCount is the number of concurrent converter/publisher goroutines
    // draining the discovery work queue. Zero means MeshSync's built-in default.
    // +kubebuilder:validation:Minimum=0
    // +kubebuilder:validation:Maximum=64
    WorkerCount int32 `json:"workerCount,omitempty" yaml:"workerCount,omitempty"`
    // QueueSize bounds the number of distinct pending object keys buffered
    // between discovery and publish. Zero means MeshSync's built-in default.
    // +kubebuilder:validation:Minimum=0
    QueueSize int32 `json:"queueSize,omitempty" yaml:"queueSize,omitempty"`
}
```
Add `Backpressure MeshsyncBackpressure `json:"backpressure,omitempty" yaml:"backpressure,omitempty"`` to `MeshSyncSpec` in both versions, and regenerate `zz_generated.deepcopy.go` (both versions) plus the `conversion.go` v1alpha1<->v1alpha2 mapping (this file already exists and handles other spec fields - extend it). This is schema-first per this feature's own instruction ("minimal config knob... should be schema-first via the CRD") and per this repo's ecosystem-wide rule that CRD/config changes are coordinated with Operator.

**Note the naming choice**: `workerCount`/`queueSize` camelCase matches the ecosystem's wire-casing contract (per `meshsync/docs/agent-instructions/naming-conventions.md`'s citation of the ecosystem-wide rule) - unlike `WatchList`'s existing `watch-list` kebab-case JSON tag in the same file, which is legacy and not to be touched.

### MeshSync CRD-config plumbing to consume the new field

**Modify `internal/config/crd_config.go`**: `GetMeshsyncCRDConfigs` currently reads only `spec.watch-list` (`crd_config.go:37-53`). Extend it to also read `spec.backpressure.workerCount`/`spec.backpressure.queueSize` from the same `unstructured.Unstructured` CRD object and populate them onto `MeshsyncConfig` (extend `internal/config/types.go`'s `MeshsyncConfig` struct with `WorkerCount int32` / `QueueSize int32`, both zero-valued/optional so an existing CR without this field behaves identically to today - see section 6 back-compat).

**Modify `internal/config/default_config.go`**: define package-level defaults, e.g. `DefaultQueueWorkerCount = 4`, `DefaultQueueSize = 2000` (concrete numbers are a starting point for review, not fixed - see Open Questions), used whenever the CRD/local config supplies a zero value.

### Meshery Server / MeshKit

No changes required. The queue does not touch the wire contract (`meshery.meshsync.core`, `pkg/model.KubernetesResource`); Server continues to consume exactly what it does today, just smoothed/paced on the producer side. Flag this explicitly in the PR description per the ecosystem-awareness rule, even though no code changes are needed there, so Server maintainers are aware publish timing/ordering characteristics change (see section 8).

## 5. Config/Schema Changes

**Schema-first per the ecosystem rule** (this is a user-tunable operational knob, CRD-consumed):
1. `meshery-operator`'s `MeshSyncSpec.Backpressure{WorkerCount, QueueSize}` is the source of truth (section 4).
2. `meshsync`'s `internal/config.MeshsyncConfig` gains matching fields, populated from the CRD in `crd_config.go`, falling back to `internal/config/default_config.go` constants when zero/absent (`GetMeshsyncCRDConfigsLocal` path, used when no CRD is present, uses the same defaults directly).
3. No `meshery/schemas` change is required - this is Operator-CRD-owned config, not a wire-message schema field (`pkg/model.KubernetesResource` is untouched), so it does not fall under the `meshery/schemas` "contract source of truth" rule the way a message-shape change would. This matches this repo's own `naming-conventions.md` framing: `pkg/model` is the wire contract needing schemas coordination; `internal/config`/CRD spec is not currently schema-sourced at all (confirmed: `internal/config/types.go` has no schemas import).
4. No new CLI flag is proposed for v1 (keep the surface area to CRD + sane built-in defaults); if a flag is added later for non-CRD (local/dev) runs, add `-workerCount`/`-queueSize` to `main.go` alongside the existing `-output`/`-outputFile`/... flags and thread through `pkg/lib/meshsync.Options`, matching that file's existing pattern.

## 6. Sequencing & Feature-Flagging

Land in this order, each independently mergeable and reviewable:

1. **PR 1 - `QueuingWriter` + tests, unwired.** Add `internal/output/queue.go`, `metrics.go`, error codes, and `internal/output/queue_test.go`. Nothing calls it yet; this PR is pure addition, zero behavior change, fully unit-testable in isolation (construct a `QueuingWriter` directly in tests with a `recordingWriter`-style fake downstream).
2. **PR 2 - wire into `pipeline.New`, defaulted to today's effective behavior.** Modify `pipeline.go`/`step.go` to wrap `ow` in a `QueuingWriter` using `internal/config` defaults. This PR changes runtime behavior (async publish) and needs the fuller test matrix (section 9) plus `verifier-meshsync` runtime verification before merge, per this repo's "Required on Every PR" rule.
3. **PR 3 - Operator CRD field + MeshSync CRD-config plumbing.** Coordinated pair: Operator CRD field lands first (or same-day), then MeshSync's `crd_config.go`/`default_config.go` consumes it. Until PR 3 merges, PR 2's defaults are the only knob - that's an acceptable interim state since the defaults are chosen to be safe (section 8/11).
4. No feature flag/env-var gate is proposed for PR 2 beyond the config defaults themselves, because: (a) the change is behind the existing `output.Writer` interface with no external API surface change, (b) `QueueSize`/`WorkerCount` defaulting to "generous enough to be a no-op under normal load, protective only under burst/broker-down" makes the change low-risk by construction rather than needing a kill switch, and (c) this repo has no existing feature-flag mechanism to piggyback on (confirmed: no flag/toggle infra found in `internal/config` beyond CRD whitelist/blacklist). If reviewers want an explicit escape hatch, the cheapest one is `QueueSize: 0` / a documented `DisableQueue bool` config bit that makes `pipeline.New` skip the `QueuingWriter` wrap entirely and pass `ow` straight through - trivial to add in PR 2 if requested.

## 7. Back-Compat & Rollout

- **Wire contract**: unchanged. Same NATS subject, same `broker.Message{ObjectType: broker.MeshSync, EventType, Object: model.KubernetesResource}` shape, same file-output YAML shape. Meshery Server requires no change.
- **CRD**: `Backpressure` is a new optional sub-struct with all-optional/zero-defaultable fields on both `v1alpha1` and `v1alpha2` - existing `MeshSync` CRs (with no `spec.backpressure`) parse identically to before (`corev1.ConfigMap`-derived `watch-list` untouched) and get MeshSync's built-in defaults. `conversion.go`'s v1alpha1<->v1alpha2 roundtrip must map the new field losslessly in both directions (add to that file's existing field-by-field copy, and to `conversion_test.go`'s roundtrip assertions - that test file already exists per the earlier grep).
- **Existing dedup writers** (`InMemoryDeduplicatorStreamingWriter`, `InMemoryDeduplicatorWriter`) are **downstream** of `QueuingWriter` in the composite chain (file-output mode) - unaffected. Broker-mode gains queuing but still has no dedup decorator, consistent with today.
- **Ordering semantics change** (flagged clearly, not hidden): today, ordering is strictly global-per-informer-goroutine (single-threaded event delivery = total order across all keys of one GVR). After this change, ordering is **per-key strict** (a worker never processes two events for the same key out of order, guaranteed by the workqueue's own "don't re-add a key that's currently being processed until `Done`" semantics) but **global order across different keys is no longer guaranteed** once `WorkerCount > 1`, because two different objects' events can be picked up by two different workers and published in either order. This is called out prominently to Server maintainers (per the ecosystem cross-repo-consumer rule) even though no code change is needed there - if Server's consumer logic ever assumed global publish order (it should not, since NATS itself doesn't guarantee cross-subject/multi-publisher total order today either, but worth confirming) this is the moment to check.
- **Rollout**: standard - ship in a MeshSync point release; Operator's CRD change is additive (new optional field) so old MeshSync binaries + new Operator, and new MeshSync binaries + old Operator (field simply absent, defaults apply), both work. No coordinated simultaneous-deploy requirement, unlike a breaking wire-model change.

## 8. Risks / Failure Modes / Perf & Scale

**Memory under burst**: bounded by `latest` map size (at most one entry per distinct in-flight key = at most `QueueSize` entries, since a key is only in `latest` while also present in the workqueue) times `sizeof(model.KubernetesResource)` (includes full spec/status - potentially large for CRDs with big specs, e.g. a `Rollout` or `Certificate`). A mass-rollout burst (task's explicit example) touches many distinct Pods/ReplicaSets - each is a **distinct key**, so coalescing does not shrink burst memory to zero, only prevents the *same* key from queuing N times. Size `QueueSize` with this in mind (section 11 default proposal: start at 2000 keys, revisit with a real memory measurement under the `verifier-meshsync` load scenario in section 9).

**Overflow/backpressure policy - block informer vs bounded-drop, decided per event type**:
- **DELETE: never drop, never block indefinitely.** A dropped delete is a permanent state-divergence bug (Server would show a resource that no longer exists, with no future event to correct it, since there's nothing left in the cluster to re-list). Policy: on `queue.Add(key)` for a Delete when the queue is at capacity, **do not drop** - instead block the enqueue call with a bounded timeout (e.g. a few seconds) logging a Warn if it takes a while, and only truly block (accepting informer stall) past that as a last resort, since a stalled informer is recoverable (catches up once the broker recovers) whereas a lost delete is not. In practice, `workqueue.TypedRateLimitingInterface` has no built-in "bounded" `Add` - it is unbounded by default. **Bounding is enforced at the `QueuingWriter.enqueue` layer**, not by the workqueue itself: track `len(w.latest)` (a proxy for distinct-keys-pending, cheaper than `queue.Len()` under the same lock) and, for Add/Update only, if pending count exceeds `QueueSize`, apply the drop policy below instead of calling `queue.Add`.
- **ADD/UPDATE: bounded drop with metrics, never block the informer goroutine.** This is where "drop the oldest queued key's pending state and don't enqueue this one" or, simpler and safer, "drop the incoming event and rely on the next resync/relist to catch up" is acceptable, because Add/Update events are idempotent with respect to a future full relist (the periodic resync essentially re-delivers a snapshot of everything the informer holds - see `docs/agent-instructions/architecture.md`'s mention of resync). Concretely: if `len(w.latest) >= QueueSize` and this is not a Delete, **do not enqueue**, increment a `drops_total{reason="queue_full", kind=...}` metric, log at Warn with rate-limiting (avoid log-storm during a sustained overflow - reuse this repo's existing `jitter`/backoff pattern from `meshsync/handlers.go` conceptually, or simply a "log every Nth drop" counter), and return `nil` from `Write` (not an error - the informer's own error-handling path already just logs-and-continues today, so surfacing an error here would be a behavior change with no consumer; instead the **metric** is the signal, deliberately pairing with the planned `/metrics` feature per the task).
- This asymmetric policy (block-with-timeout for deletes, drop-with-metric for add/update) is the single most important architectural decision in this design and must be documented prominently in code comments at the `enqueue` function, since it is easy for a future maintainer to "simplify" it into one uniform policy and silently reintroduce delete-loss.

**Ordering**: per-key strict (guaranteed by workqueue semantics - a key cannot be `Get()` by a second worker while still outstanding from a first, until `Done`); global order not guaranteed across keys with `WorkerCount > 1` (see section 7). Within a key, the "latest wins" coalescing map means an intermediate state can be skipped entirely (e.g. Update A -> Update B -> Update C collapses to publishing only C) - this is the intended smoothing behavior, not a bug, but must be called out because it means **not every Update event that fired at the API server necessarily reaches the broker** under sustained load; only Add and Delete for a given object are guaranteed to eventually publish (Delete because of the never-drop policy; Add because it's the first sighting and, if dropped under overflow, the next resync/relist will re-deliver it as an Add-equivalent anyway once queue pressure clears - though this is worth an explicit unit test, see section 9).

**Broker down for an extended period**: `QueueSize` is finite, so this design does **not** turn into unbounded memory growth even if the broker is down for hours - it degrades to "drop Add/Update events past capacity, metric climbing, Deletes still get through (slowly, backing off) or eventually the block-with-timeout-then-block-anyway path engages for deletes specifically." This is a deliberate, bounded degradation, not a crash-preventing safety valve that itself becomes unbounded.

**Deadlock/goroutine-leak risk**: the `Shutdown(drain: true)` path must have a **hard timeout**, not rely purely on `ShutDownWithDrain()`'s "wait for in-flight `Done()` calls" semantics, because a worker blocked inside `w.real.Write` (i.e. blocked on a still-hanging broker publish) would never call `Done()`, and `ShutDownWithDrain` would then hang the entire resync/shutdown path indefinitely - directly reintroducing the exact stall this feature exists to eliminate, just moved to the shutdown path. Concretely: `Shutdown` calls `ShutDownWithDrain()` in a goroutine and races it against a `time.After(drainTimeout)`, logging a Warn and proceeding with process/resync teardown if the timeout wins (in-flight workers are abandoned; their goroutines will still exit once their blocked `Write` eventually returns or errors, since nothing double-closes the queue).

**Race conditions**: the `latest` map's lock (`w.mu`) must be held across the "check pending count for overflow decision" + "write to map" + "call `queue.Add`" sequence as one atomic unit per key, or two concurrent `Write` calls for the same key could both pass the overflow check and only one wins in the map, or - worse - the pending-count check and the map write could observe inconsistent state. `go test -race` (already this repo's default test invocation, per `make test`) is the primary tool to catch this; write an explicit concurrent-writers stress test (section 9).

## 9. Test Plan

**Unit (`internal/output/queue_test.go`, new)**:
- Single key, three rapid `Write` calls (Add, Update, Update) before any worker drains -> exactly one downstream `Write` call, carrying the **last** payload (assert on `recordingWriter`-equivalent fake's captured object, not just call count).
- Delete after Add for the same key, both enqueued before drain -> both eventually published (delete must not be coalesced away by a later add-of-a-different-instance; use distinct-enough test fixtures to make this unambiguous) - and specifically assert Delete's payload is the pre-deletion object (side-stash correctness).
- Overflow: fill `QueueSize` with distinct Add keys, assert the `(QueueSize+1)`th Add is **dropped** (downstream never sees it) and the drop metric increments; then send a Delete at the same overflow point and assert it is **not** dropped (published, possibly after a bounded wait) - this is the single highest-value test in the whole plan given section 8's core risk.
- Downstream `Write` returns an error -> assert `AddRateLimited` requeues the key (observable via the fake queue or via a second successful attempt eventually landing) and the metric records a retry; assert the key is not silently dropped after one failure.
- `Shutdown(drain: true)` with all workers idle -> returns promptly, no dropped in-flight work.
- `Shutdown(drain: true)` with a worker deliberately blocked in a fake downstream `Write` -> returns after `drainTimeout`, not indefinitely (proves the hard-timeout requirement in section 8).
- Concurrent-writers stress test: many goroutines calling `Write` on overlapping keys simultaneously while workers drain concurrently, run under `-race` - asserts no panic, no lost Deletes, final state converges (every key's last-written state is what's eventually published or explicitly counted as dropped).
- `queuedKeyFor` derivation: correct key for objects with/without namespace (cluster-scoped vs namespaced), and stability (same logical object always yields the same key across repeated calls).

**Unit (`internal/pipeline/pipeline_test.go` and `step_test.go`, extend existing)**:
- `pipeline.New(...)` with the queue wired: assert the writer handed to `RegisterInformer.outputWriter` is a `*output.QueuingWriter` (or behaves like one - e.g. rapid duplicate `Write` calls through the full `pipeline.New`-constructed chain collapse as expected), not the raw `ow`.
- Existing `handlers_test.go` tests (`TestDeleteFunc_Tombstone`, etc.) construct `RegisterInformer` directly with a `recordingWriter`, bypassing `pipeline.New` - these remain valid, unmodified regression tests of handler-level logic (tombstone unwrap, RV suppression) that must keep passing exactly as-is, proving the queue wrap didn't disturb the handler layer.

**Race**: `make test` already runs `go test -failfast --short ./... -race` - the new tests above must pass under `-race` as the bar, per `docs/agent-instructions/testing.md`.

**Integration** (`integration-tests/`, extend `meshsync_as_binary_with_k8s_cluster_test_cases_mode_a_broker_test.go` or add a new burst-scenario case per that file, per this repo's convention of adding cases to existing scenario files rather than new top-level test files): a scenario that creates many resources rapidly (simulating a mass rollout) against the kind cluster + NATS docker-compose stack, and asserts every Add/Delete eventually appears in the consumed broker stream (Updates may legitimately coalesce, so assert on Add/Delete completeness, not exact event-count parity).

**Runtime verification under load** (`verifier-meshsync` skill, per the task's explicit callout): 
1. `up` + `run` per the skill's quick start.
2. Drive a burst: script rapid `kubectl scale`/rollout-restart on a Deployment with many replicas, or `kubectl label` loop across many Pods, to generate a genuine multi-key Update burst.
3. Additionally simulate broker unavailability: since the skill's default is file-output mode (no NATS needed), for this specific verification run in **broker mode** against a NATS instance you can pause/kill mid-run (e.g. `docker stop` the NATS container from `make nats-run`), then resume it, and confirm via debug log that: informer ADD/UPDATE/DELETE log lines (`Received ADD event for` etc., per the skill's existing signal table) keep appearing **during** the NATS outage (proving the informer goroutine is not stalled), a queue-depth/drop signal appears (once the metrics hooks exist) or is inferable from retry-warn log lines, and after NATS resumes, no Delete was lost (cross-check final cluster state against what actually got published, e.g. via the file-mode snapshot in parallel or a broker-side subscriber).
4. Reconcile counts per the skill's existing "airtight reconciliation" convention (section 96-99 of that skill doc): burst events driven vs. events observed published vs. events observed dropped (metric) should sum correctly.

## 10. Effort Estimate & Phasing

- **PR 1** (`QueuingWriter` core + metrics interface + unit tests): ~2-3 days, one engineer. Self-contained, no wiring risk.
- **PR 2** (wire into `pipeline.New`/`step.go`, extend pipeline-level tests, `verifier-meshsync` runtime pass, integration burst test): ~3-4 days including the runtime-verification cycle (this is where the async-behavior risk actually surfaces and needs careful manual + automated validation of the overflow/delete-safety policy).
- **PR 3** (Operator CRD field both API versions + deepcopy + conversion + MeshSync CRD-config plumbing + defaults): ~1-2 days, mostly mechanical (mirrors the existing `MeshsyncBroker` sub-struct pattern) but requires an Operator-repo PR review cycle in parallel/coordination.
- **Total**: roughly 1.5-2 engineer-weeks including review cycles across two repos, before considering the paired `/metrics` feature (out of scope here beyond the `Metrics` interface seam).

Phase as: PR1 -> PR2 (can ship and get real-world burst signal via logs/retries even before PR3's tunables exist, using built-in defaults) -> PR3 whenever Operator review lands, non-blocking of PR1/2's value delivery.

## 11. Open Questions

1. **Default `QueueSize`/`WorkerCount` values**: proposed starting points (2000 keys, 4 workers) are estimates, not measured. Needs a real memory/throughput measurement under the runtime-verification burst scenario (section 9) before finalizing - flag this as a to-be-tuned-post-merge parameter, not a blocking design gap.
2. **Should the `MetricsProvider`/`Metrics` seam live in MeshKit instead of MeshSync-local?** The task explicitly asks this. Recommendation: **keep it MeshSync-local for now** (a small interface in `internal/output/metrics.go`), because (a) no other MeshKit consumer currently needs a workqueue-backed writer - MeshKit's `broker/` package is a thin transport wrapper, not a discovery-pipeline component, so there's no proven second consumer yet to justify the abstraction cost, and (b) premature generalization into MeshKit before the paired `/metrics` feature's exposition format (Prometheus? something else?) is decided risks designing the wrong shared interface. Revisit MeshKit extraction once `/metrics` lands and/or if a second MeshKit-ecosystem component (e.g. a future Meshery Server-side consumer with its own backpressure need) wants the same pattern - at that point, promote the `QueuingWriter` pattern (not necessarily the MeshSync-specific `output.Writer`-typed wrapper, but the generic "keyed coalescing queue + worker pool + asymmetric overflow policy" pattern) into `meshkit/utils` as a reusable generic type.
3. **Should `WorkerCount`/`QueueSize` be per-pipeline (per resource kind) or global to the whole `QueuingWriter`?** This blueprint proposes **one shared `QueuingWriter` instance for the entire discovery run** (constructed once in `pipeline.New`, shared across all `RegisterInformer` steps via the single wrapped `ow`), not one per resource kind, because per-kind queues would multiply `WorkerCount` goroutines by the number of watched kinds (potentially dozens) with no clear benefit, and the coalescing/overflow policy is naturally global (a burst is usually cross-kind - Pods, ReplicaSets, Endpoints all churn together during a rollout). If review pushes back wanting per-kind isolation (e.g. so a CRD with huge specs can't starve Pod events of queue capacity), that's a valid alternative worth a follow-up design note, but adds meaningful complexity (N queues, N-times worker pools) not justified by anything found in the current codebase's resource-kind priorities.
4. **Drain timeout value** (section 8's hard-timeout requirement) - propose defaulting to a few seconds, aligned with this repo's existing `debounce(time.Second*5, ...)` resync-debounce constant in `meshsync/handlers.go:62`, but needs explicit review sign-off as a named constant, not silently inherited.
5. **Does dropping an Add under overflow ever leave Server permanently unaware of a resource** (not just delayed until next resync)? This blueprint's safety argument leans on "the next periodic resync/relist re-delivers everything as an Add-equivalent." Confirm this is actually true end-to-end (does MeshSync's informer resync genuinely re-fire `AddFunc` for every already-known object, or only for genuinely-new ones since last sync?) with a targeted unit/integration test before relying on it as the correctness backstop for the drop policy - this is flagged as a **must-verify-before-merge** item for PR 2, not just an open question, since the whole "drop is safe" argument in section 8 depends on it.

---

**Key files referenced** (all absolute paths):
- `internal/pipeline/handlers.go`
- `internal/pipeline/pipeline.go`
- `internal/pipeline/step.go`
- `internal/pipeline/error.go`
- `internal/pipeline/handlers_test.go`
- `internal/output/output.go`
- `internal/output/broker.go`
- `internal/output/composite.go`
- `internal/output/processor.go`
- `internal/output/file.go`
- `internal/output/inmemory_deduplicator.go`
- `internal/output/inmemory_deduplicator_streaming.go`
- `internal/config/types.go`
- `internal/config/crd_config.go`
- `internal/config/default_config.go`
- `pkg/lib/meshsync/meshsync.go`
- `pkg/lib/meshsync/health.go`
- `meshsync/handlers.go`
- `meshsync/meshsync.go`
- `helpers/component_info.json`
- `docs/agent-instructions/architecture.md`
- `docs/agent-instructions/errors.md`
- `docs/agent-instructions/naming-conventions.md`
- `.claude/skills/verifier-meshsync/SKILL.md`
- `meshkit/broker/broker.go`
- `meshkit/broker/nats/nats.go`
- `meshery-operator/api/v1alpha1/meshsync_types.go`
- `meshery-operator/api/v1alpha2/meshsync_types.go`

Note: several stray `.claude/worktrees/agent-*/` directories exist under `.claude/worktrees/` (leftover from other agent sessions, containing copies of `pkg/lib/meshsync/health.go`) - not part of this blueprint, flagged only because they surfaced in a glob and are worth cleaning up separately.
