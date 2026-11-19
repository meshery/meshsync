# Architecture

## Overview

MeshSync is a standalone Go binary, one instance per managed Kubernetes cluster. It has no HTTP API and no persistent store of its own: it watches the API server via dynamic informers, converts each object into a canonical model, deduplicates, and publishes the result over NATS (default) or to a snapshot file.

```
main.go --parses CLI flags--> pkg/lib/meshsync.Run(...)
                                      |
                                      v
                          discovery.Registry.Build(Options.Platform)
                                      |
                                      v
                 internal/discovery/kubernetes.Discoverer (Watch)
                                      |
                                      v
                              meshsync.Handler (meshsync/meshsync.go)
                                      |  dynamic informer factory (client-go)
                                      v
                          internal/pipeline.New(...)  <-- rebuilt every run/resync
                                      |
                    +-----------------+------------------+
                    | Global-resource  | Local-resource    | StartInformers
                    | discovery stage  | discovery stage   | stage
                    +-----------------+------------------+
                                      |
                                      v  discovery.EmitFunc (model.Resource)
                                      v
                          internal/output.Writer  (broker | file | composite)
                                      |            with internal/output dedup
                                      v
                     NATS broker (Meshery Server consumes it) or snapshot file
```

## Entry Point and Handler

- `main.go` parses flags (`-output`, `-outputFile`, `-outputNamespaces`, `-outputResources`, `-stopAfter`) and calls `pkg/lib/meshsync.Run(...)`, which builds the platform's `Discoverer` through `internal/discovery` (see below).
- `meshsync.Handler` (`meshsync/meshsync.go`) holds the config, logger, broker handle, dynamic informer factory, kube client, channel pool, emit function (`discovery.EmitFunc`, defaulting to the output writer passed to `New`), and output-filtration config. `meshsync.New(...)` wires them together and derives the cluster ID via `pkg/utils.GetClusterID`.
- `GetDynamicInformer` builds a `dynamicinformer.DynamicSharedInformerFactory`. Resource filtering happens in the watch-list config (`internal/config/crd_config.go` decides which informers get registered); the factory's list-options hook (`GetListOptionsFunc`) is a deliberate no-op.

## Discovery Abstraction (`internal/discovery`)

- `discovery.Discoverer` is the seam between a platform's discovery and the output plumbing: `Platform()`, `ScopeID(ctx)`, `Discover(ctx, emit)`, `Watch(ctx, stopCh, emit)`, `Shutdown()`. Kubernetes is the only implementation today; Docker and public clouds are planned as further implementations (see [FD6](../design/fd6-non-kubernetes-discovery.md)).
- `discovery.EmitFunc` receives every discovered resource as a `model.Resource` envelope (`pkg/model/resource.go`): platform-agnostic fields (`platform`, `scopeId`, `kind`, `name`, ...) plus, for Kubernetes, the full `KubernetesResource` in `Resource.Kubernetes`. The envelope is a hand-written mirror of the planned schemas `resource` construct.
- `discovery.WriterEmitFunc` adapts an `output.Writer`: it passes `Resource.Kubernetes` to the writer unchanged, so broker messages and snapshot files keep the `KubernetesResource` wire shape. Widening `output.Writer` to `model.Resource` changes that wire shape and must be coordinated with Meshery Server.
- `scopeId` generalizes `cluster_id`. For Kubernetes both carry the same value (the `kube-system` namespace UID) for one migration window; do not drop either until consumers have migrated.
- `discovery.Registry` maps a platform name to a factory. `Run` creates one per call and `Build` constructs a fresh `Discoverer` each time - there is no package-level registry or shared Discoverer state. `pkg/lib/meshsync.Options.Platform` (`WithPlatform`, default `kubernetes`, validated against `AllowedPlatforms`) selects the platform; there is no CLI flag for it.
- `internal/discovery/kubernetes` wraps `meshsync.Handler` without changing it: `Watch` starts the CRD watch (when the MeshSync CRD exists) and the broker request listeners (broker mode only), then runs `Handler.Run` until `channels.Stop` closes. `Discover` returns `ErrDiscoverNotSupported` because informers list everything before watching, and `Shutdown` is a no-op because `Handler.Run` tears the informers down itself when `channels.Stop` closes.

## Discovery Pipeline (`internal/pipeline`)

- Built on `github.com/myntra/pipeline`. `pipeline.New(...)` constructs **fresh stages on every call** - it runs once at startup and again on every resync, so stages/steps must never be cached in package-level state (a shared stage would retain a shut-down informer factory and a closed stop channel from a prior run).
- Three stages, run in order: **global-resource discovery**, **local-resource discovery** (both register one informer step per configured resource kind, skipping any kind excluded by `outputFiltration.ResourceSet`), then **StartInformers** (starts every registered informer against the given stop channel).
- `internal/pipeline/step.go` defines the per-resource-kind informer registration step; `internal/pipeline/handlers.go` are the Add/Update/Delete event callbacks that convert an informer event into a `model.KubernetesResource`, wrap it in a `model.Resource`, and hand it to the injected `discovery.EmitFunc`.

## Config (`internal/config`)

- `config.go` / `default_config.go` / `crd_config.go` define the discoverable resource set (global vs. local/namespaced), whitelist/blacklist, and pluralization (`pluralise.go`) needed to map a Kind to its API resource.
- The watch-list blacklist also applies to CRDs found at runtime: `Run` copies it into `meshsync.Handler.BlackList`, and the CRD watch (`updatePipelineConfig`) never registers a blacklisted CRD. The whitelist does not restrict CRD discovery.
- `OutputFiltrationContainer` / `OutputResourceSet` (referenced from `meshsync.Handler`) carry the `-outputNamespaces` / `-outputResources` CLI restrictions through to the pipeline and output writer.

## Output (`internal/output`)

- `output.Writer` is the single interface consumed by the pipeline: `Write(obj model.KubernetesResource, evtype broker.EventType, config config.PipelineConfig) error`.
- Implementations: `broker.go` (publishes to the MeshKit `broker.Handler`, i.e. NATS), `file.go` (writes a cluster snapshot via `internal/file`), `composite.go` (fans out to multiple writers - used when both broker and file output are needed).
- `inmemory_deduplicator*.go` suppresses redundant republishes of unchanged resources; `processor.go` is the shared write-path plumbing.

## Model (`pkg/model`)

- `model.KubernetesResource` (plus `KubernetesResourceObjectMeta`, `KubernetesResourceSpec`, `KubernetesResourceStatus`, `KubernetesKeyValue`) is MeshSync's canonical wire/DB shape - a local Go/GORM struct, not generated from `github.com/meshery/schemas`. See [naming conventions](naming-conventions.md) for the casing implications.
- `model_converter.go` / `preprocessor.go` convert a raw `unstructured.Unstructured` informer object into this model; `exec.go` / `log.go` / `process.go` handle exec-stream and log-stream requests routed in over the broker (see `meshsync/exec.go`, `meshsync/logstream.go`).

## Channels (`internal/channels`)

- `channel.go` / `generic.go` / `system.go` / `broker.go` define small typed channels (e.g. `StructChannel`) used for coordination (stop signals, broker request/response) between the handler and the pipeline - not a general pub/sub system.

## Interactive Sessions (exec / log stream)

- Meshery Server routes interactive `kubectl exec` and pod-log requests to MeshSync over the broker; `meshsync/exec.go` (`processExecRequest`) and `meshsync/logstream.go` (`processLogRequest`) start one long-lived goroutine per request, keyed by a request id.
- These per-session channels live in a **`sync.Mutex`-guarded `sessions` map** on the Handler (`meshsync/sessions.go`), deliberately separate from `channelPool`, which holds only the fixed system channels (`Stop`/`OS`/`ReSync`) and is read-only after construction. Keeping them apart avoids the concurrent map read/write panic that occurred when session goroutines mutated the same map other goroutines ranged.
- An exec session subscribes to its own `input.<id>` subject (client keystrokes) via `SubscribeWithChannel`. On teardown - stream EOF/error, an explicit stop request, or the global `Stop` - `terminate()` runs once (guarded by a `sync.Once`) and calls `broker.Handler.Unsubscribe("input.<id>")`, which releases the subscription and the broker's delivery goroutine. Before MeshKit exposed `Unsubscribe` (v1.0.22), the subscription could not be torn down and each session parked a drain goroutine that never exited, leaking a goroutine and a subscription per session.

## Deployment Topology

- Meshery Operator's `MeshSync` controller (`meshery/meshery-operator`, `pkg/meshsync/meshsync.go`) renders this binary as a `Deployment` and injects `BROKER_URL` pointing at the Broker's derived NATS endpoint. See that repo's architecture doc for the reconcile side.
- Meshery Server subscribes to the NATS subjects MeshSync publishes on and persists/serves the resulting resource state; the `pkg/model.KubernetesResource` shape is the de facto contract between the two.
- Coordinate CLI flag, config-schema, or model changes with both `meshery-operator` and `meshery` (server) - this repo does not own the full contract in isolation.

## Further Reading

- [design-spec: MeshSync infrastructure synchronization](../design-spec_meshsync-infrastructure-synchronization.md) - discovery concepts, tiered discovery, composite prints, user stories
- [design-spec: embedded MeshSync](../design-spec_embedded-meshsync.md) - proposal for running MeshSync in-process inside Meshery Server
