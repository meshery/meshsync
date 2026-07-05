# IMPLEMENTATION BLUEPRINT: Non-Kubernetes / Multi-Platform Discovery for MeshSync

## 1. Goal & Gap

**Goal:** Let MeshSync discover and continuously synchronize resources on platforms other than Kubernetes (Docker first; public clouds later) into Meshery Server's existing Connection state machine and UI, via an infrastructure-agnostic object model - without a big-bang rewrite.

**Gap:** Every layer of MeshSync today assumes Kubernetes:

- Discovery: `dynamicinformer.DynamicSharedInformerFactory` + `unstructured.Unstructured` (`meshsync/meshsync.go`, `internal/pipeline/`).
- Resource identity: `internal/config.PipelineConfig.Name` is a Group/Version/Resource string parsed with `k8s.io/apimachinery/pkg/runtime/schema.ParseResourceArg` (`internal/pipeline/step.go:46`).
- Wire model: `pkg/model.KubernetesResource`, whose method `ParseList` (`pkg/model/model_converter.go`) marshals an `unstructured.Unstructured` to JSON and unmarshals into the K8s-shaped struct.
- Cluster/connection identity: `pkg/utils.GetClusterID` derives an ID from the `kube-system` namespace's UID (`pkg/utils/utils.go:27`) - there is no non-K8s equivalent identity concept anywhere.
- Deployment: Meshery Operator's `MeshSync` CRD (`meshery-operator/api/v1alpha1/meshsync_types.go`) has fields `WatchList corev1.ConfigMap`, `Broker`, `Version`, `Size` - no platform/mode field at all, and `pkg/meshsync/meshsync.go` renders a literal `k8s.io/api/apps/v1.Deployment` via `sigs.k8s.io/controller-runtime`. Operator cannot deploy or manage anything non-K8s.

Meshery Server, by contrast, already generalizes past Kubernetes at the Connection layer (Section 2). The job is to bring MeshSync's discovery side up to that same level of generality, incrementally.

## 2. Current State Per Repo (grounded)

### MeshSync (`meshsync`)

- `main.go` -> `pkg/lib/meshsync.Run(...)` (`pkg/lib/meshsync/meshsync.go`) builds a `mesherykube.Client`, then `meshsync.New(...)` (`meshsync/meshsync.go:55`), which calls `GetDynamicInformer` (K8s dynamic informer factory) unconditionally.
- `internal/pipeline.New(...)` (`internal/pipeline/pipeline.go:15`) builds **fresh stages every call** (global-resource stage, local-resource stage, StartInformers stage) from `internalconfig.PipelineConfigs` - this "rebuilt per run" contract (CLAUDE.md rule 4) must be preserved by any new abstraction.
- `internal/pipeline/step.go`'s `RegisterInformer.Exec` is the single point where a `PipelineConfig.Name` becomes a live K8s informer (`schema.ParseResourceArg` + `informer.ForResource(gvr)`).
- `internal/pipeline/handlers.go`'s `publishItem` converts an `*unstructured.Unstructured` via `model.ParseList(*obj, evtype, ri.clusterID)` and calls `ri.outputWriter.Write(k8sResource, evtype, config)`.
- `internal/output.Writer` interface (`internal/output/output.go`) is **already shape-generic** except its two concrete parameter types (`model.KubernetesResource`, `config.PipelineConfig`); `BrokerWriter`, `FileWriter`, `CompositeWriter`, `Processor`, the in-memory deduplicators are all pure plumbing with zero K8s API calls inside them.
- `pkg/lib/meshsync` already runs as an in-process library (`Run(log, ...OptionSetter)`) with an `Options.OutputMode` of `broker` or `file` (`pkg/lib/meshsync/options.go`) - this is the existing "embedded" entry point per `docs/design-spec_embedded-meshsync.md`, but it is **still 100% Kubernetes underneath** (it still calls `mesherykube.New`).
- `pkg/utils.GetClusterID` (`pkg/utils/utils.go:27`) is K8s-only (reads `kube-system` namespace UID) and is the sole source of the `clusterID` string threaded through the whole pipeline and burned into `model.SetID`'s resource ID (`pkg/model/model_converter.go:110`).
- No `Discoverer`/`Platform` interface exists anywhere today (confirmed via search).

### Meshery Server (`meshery/server`)

- **Connection layer is already schema-first and platform-generic.** `models/connections/connections.go:67`: `type Connection = schemasConnection.Connection` (a direct alias of `github.com/meshery/schemas/models/v1beta3/connection.Connection`). The Connection model's `Kind` field is a free string (`kubernetes, prometheus, grafana, gke, aws, azure, slack, github` per the schema description) and `Metadata core.Map` is an open bag - non-K8s kinds are structurally supported today, just not populated.
- **State machine core is kind-agnostic.** `machines/state.go` (`StateType`, `State`, `Events`) and `machines/models.go`'s `StateMachine.SendEvent` operate purely on `connections.Connection`/`connections.ConnectionPayload` and generic `EventType`s (`Discovery, Register, Connect, Disconnect, Ignore, NotFound, Delete`). Kubernetes gets its own package (`machines/kubernetes/`) wiring K8s-specific `Action`s (`DiscoverAction`, `RegisterAction`, ...) into those generic states; `grafana` and `prometheus` are simpler (register-only) packages using `machines.DefaultConnectAction` for connect. `machines/helpers/helpers.go:37`'s `getMachine(mtype string, ...)` switch is the existing per-kind extension point (`case "kubernetes"`, `case "grafana"`, `case "prometheus"`) - a Docker/AWS machine slots in the same way.
- **`AutoRegistrationHelper.processRegistration`** (`machines/helpers/auto_register.go:57`) is the mechanism that promotes a *discovered resource* into a first-class Connection: it reads `data.Obj.ComponentMetadata["capabilities"]["connection"]`/`["urls"]`, fingerprints the connection kind via `getTypeOfConnection` (currently a hardcoded name-substring match on `meshsyncmodel.KubernetesResource`, `auto_register.go:180`), builds a `connections.ConnectionPayload`, and drives the FSM (`Register` -> `Connect`). **This is the exact seam a non-K8s discoverer's output must reach** to auto-promote e.g. a discovered Docker container running Prometheus into a `prometheus` connection - but today its only input type is `meshsyncmodel.KubernetesResource`.
- **Server directly imports MeshSync's Go package for the resource payload**, not a schemas type: `models/meshsync_events.go:16` (`meshsyncmodel "github.com/meshery/meshsync/pkg/model"`) and `machines/helpers/auto_register.go:25` both decode broker/store messages straight into `meshsyncmodel.KubernetesResource`. This is the opposite of schema-first for the *resource* payload (as distinct from the Connection object, which is schema-first) - a pre-existing architecture gap, confirmed by MeshSync's own `docs/agent-instructions/naming-conventions.md`.
- **`meshsync_deployment_mode` already exists as a real, working extension point** for deployment topology: `schemas/models/v1beta1/connection/connection_helper.go` originally defined `MeshsyncDeploymentMode{Operator, Embedded, Undefined}` keyed at `Connection.Metadata["meshsync_deployment_mode"]`; this has since been **intentionally moved out of schemas into `meshery/server/models/connections/meshsync_deployment_mode.go`** (per that file's own header comment) as Meshery-domain logic, with schemas remaining the pure wire contract (the open `metadata` map). Server's `handlers/k8sconfig_handler.go:138-153` (`getMeshsyncModeForContext`) reads a per-context or global mode when a Kubernetes connection is registered via `SaveK8sContext` + `InitializeMachineWithContext(..., "kubernetes", kubernetes.AssignInitialCtx)`. **Update (2026-07-04): `pkg/lib/meshsync.Run(...)` is now invoked in-process by `meshery/server`** - `models/meshery_controllers.go` (`AddMeshsyncDataHandlers`) runs MeshSync as a library over an in-process channel broker when a connection's mode is embedded, and embedded is the server's default deployment mode (`MeshsyncDeploymentModeDefault`). Embedded is a live code path in Server that this design can build on directly.
- No `docker`, `aws`, or other non-K8s connection kind has any code today (only icon-asset JSON under `meshery/models/meshery-dev-icons/`) and no model in `meshery/models/*` has a populated `connections/` folder (the `ConnectionDefinition` per-model registry construct referenced in `schemas/schemas/constructs/v1beta3/connection/api.yml:528` exists as scaffolding but is unused).

### meshery/schemas (`schemas`)

- `models/v1beta3/connection/connection.go` (generated from `schemas/constructs/v1beta3/connection/connection.yaml`) is the canonical `Connection` struct: `ID, Name, CredentialID, Type, SubType, Kind, Metadata core.Map, Status, UserID, CreatedAt/UpdatedAt/DeletedAt, Environments, SchemaVersion`. `Type`/`SubType`/`Kind` are free strings by design (enum values are documentation-only in the description field, not JSON-schema `enum:` constraints for `type`/`sub_type`/`kind` - only `status` has a real enum). This is deliberately open for exactly this kind of extension.
- `ConnectionDefinition` (`schemas/constructs/v1beta3/connection/api.yml:527`) - "an uninitialized connection, authored per-model (in a model's `connections/` folder)...conforms to the connection schema; the dynamic, kind-specific shape is carried in `metadata`" - is the sanctioned mechanism for declaring a new connection kind's shape/credential-schema/transition-map, but is unused in practice today.
- `K8sContext` (`schemas/constructs/v1beta3/connection/api.yml:1044`) carries a `deploymentType` field ("How Meshery is deployed relative to the cluster (e.g. in_cluster, out_of_cluster)") - a precedent for a per-kind "how is this being observed" discriminator living in a kind-specific projection type, reinforcing that a **non-K8s discoverer's topology metadata belongs in kind-specific metadata/projections, not in the generic `Connection` schema**.
- No platform-agnostic *resource* model (as distinct from *connection*) exists in schemas today - `component.ComponentDefinition` is the closest analog (a registry entity for a *type* of resource within a *model*), but there is no schema for a discovered resource *instance* (MeshSync's `KubernetesResource` has no schemas counterpart at all).

### Meshery Operator (`meshery-operator`)

- `pkg/meshsync/meshsync.go` renders `k8s.io/api/apps/v1.Deployment` via `sigs.k8s.io/controller-runtime/pkg/client`; `api/v1alpha1/meshsync_types.go`'s `MeshSyncSpec{WatchList, Broker, Version, Size}` has no mode/platform field. **Operator is architecturally inapplicable to any out-of-cluster target** - it only exists to run inside a cluster it's deploying into. Confirmed, not assumed.

### MeshKit (`meshkit`)

- `broker` package: `broker.Message{ObjectType, EventType, Object}`, `ObjectType` constants (`broker.MeshSync = "meshsync-data"`, etc.) are already payload-type-agnostic - the `Object` field is `interface{}`, so a non-K8s resource can ride the exact same broker message shape without any MeshKit change.
- `errors` package: MeshSync's mandated error-builder convention (see `docs/agent-instructions/errors.md`); reused as-is for new error paths.
- `orchestration.ResourceSourceDesignIdLabelKey = "design.meshery.io/id"` (`meshkit/orchestration/design.go`) is a K8s-label-shaped design-tracing key consumed in `model_converter.go:38` - generalizes reasonably (Docker labels, cloud tags both exist) but needs an abstraction, not a rewrite, in the adapter layer.

## 3. Proposed Architecture

### 3.1 The enabling refactor: `Discoverer` interface, K8s as first adapter

Introduce a new package, **`internal/discovery`** (sibling to `internal/pipeline`, `internal/output`), defining the seam that isolates platform specifics from pipeline/output/broker plumbing:

```go
// internal/discovery/discoverer.go
package discovery

type Discoverer interface {
    // Platform returns a stable, lowercase discriminator ("kubernetes", "docker", "aws"),
    // used as model.Resource.Platform and to key metrics/logs.
    Platform() string

    // ClusterID (poor name retained only as the existing wire field name -
    // see 5.2) returns this discoverer's scope identity: a Kubernetes cluster
    // UID, a Docker daemon ID, an AWS account+region, etc. Must be stable
    // across restarts of MeshSync against the same target.
    ScopeID(ctx context.Context) (string, error)

    // Discover performs one full inventory pass, calling emit for every
    // resource currently visible. Used for the initial sync / snapshot mode
    // and for resync after a config change.
    Discover(ctx context.Context, emit EmitFunc) error

    // Watch starts continuous change notification (informer-equivalent:
    // Kubernetes watch, Docker events API, cloud poll-diff) until stopCh
    // closes, calling emit for every Add/Update/Delete. Implementations that
    // have no native push/watch API (most cloud inventory APIs) implement
    // this as a poll-diff loop against Discover.
    Watch(ctx context.Context, stopCh <-chan struct{}, emit EmitFunc) error

    // Shutdown releases any held resources (informer factories, connections).
    Shutdown()
}

// EmitFunc is what a Discoverer calls per discovered/changed resource; it is
// the new join point to internal/output.Writer, replacing the direct
// model.ParseList + outputWriter.Write call in pipeline/handlers.go today.
type EmitFunc func(res model.Resource, evtype broker.EventType) error
```

`Discoverer` is deliberately **coarse** (discover/watch/emit), not resource-kind granular, because MeshSync's existing per-resource-kind fan-out (`PipelineConfig` list -> one informer per GVR) is itself a Kubernetes idiom (informers are cheap and per-GVR in client-go); Docker's events API and cloud list APIs do not decompose the same way. Each concrete `Discoverer` owns its own internal fan-out strategy.

### 3.2 Where Kubernetes becomes an adapter

`internal/discovery/kubernetes/` becomes the first (and, for a long transition, only working) concrete `Discoverer`, wrapping - not rewriting - the existing `meshsync.Handler` + `internal/pipeline` machinery:

- `Discoverer.Discover`/`Watch` for the Kubernetes adapter delegate straight into today's `pipeline.New(...)` + `myntra/pipeline.Pipeline.Run()` (the existing three-stage discovery). This is the crux of "incremental, not big-bang": the K8s adapter is a **thin wrapper that changes nothing about how Kubernetes discovery actually works**, it just relocates the `meshsync.Handler`'s direct instantiation behind the new interface.
- `ScopeID` becomes a package-qualified rename of today's `pkg/utils.GetClusterID` (kept as a re-exported wrapper for compatibility, see 6.3).
- The emit callback inside `internal/pipeline/handlers.go`'s `publishItem` changes from calling `ri.outputWriter.Write(k8sResource, ...)` directly to calling the injected `EmitFunc`, which the Kubernetes adapter wires straight to the still-existing `output.Writer`. No behavior changes for the K8s path; the call graph is reshaped, not reimplemented.

### 3.3 Data flow (target state, K8s adapter shown, Docker adapter identical shape)

```
pkg/lib/meshsync.Run(options)
        |
        v
  discovery.Registry.Build(options.Platform)  <-- new: selects + constructs one Discoverer
        |
        +--> discovery/kubernetes.New(...)  (wraps meshsync.Handler + internal/pipeline, unchanged internals)
        |         |
        |         v
        |   internal/pipeline.New(...) --[fresh per run, unchanged]--> myntra/pipeline stages
        |         |
        |         v (EmitFunc, replaces direct outputWriter.Write call site)
        |   model.Resource{Platform:"kubernetes", ...KubernetesResource embedded...}
        |
        +--> discovery/docker.New(...)   (new, Phase 1)
                  |
                  v
            Docker events API + periodic List (containers/images/networks/volumes)
                  |
                  v (EmitFunc)
            model.Resource{Platform:"docker", ...}
        |
        v
  internal/output.Writer  (UNCHANGED: BrokerWriter | FileWriter | CompositeWriter + dedup)
        |
        v
  NATS broker (meshery.meshsync.core) or file
        |
        v
Meshery Server: models/meshsync_events.go MeshsyncDataHandler
        |
        +--> DB persistence (existing meshsyncmodel.KubernetesResource path, extended - 5.3)
        |
        +--> machines/helpers/auto_register.go AutoRegistrationHelper
                  |
                  v (extended fingerprint, 5.4)
            machines.StateMachine (existing, kind-agnostic) --> connections.Connection (kind="docker"/"prometheus"/...)
                  |
                  v
            Meshery UI (Connections page) - no change needed, already kind-generic
```

## 4. Per-Repo Changes

### MeshSync

| File | Change |
|---|---|
| `internal/discovery/discoverer.go` (new) | `Discoverer` interface, `EmitFunc`, `Registry` (a small `map[string]func(...) (Discoverer, error)` factory keyed by platform name). |
| `internal/discovery/kubernetes/kubernetes.go` (new) | Wraps existing `meshsync.Handler` (`meshsync/meshsync.go`) + `internal/pipeline`. Constructor takes the same args `meshsync.New` takes today. `Discover`/`Watch` call the existing pipeline-build-and-run path; `ScopeID` wraps `pkg/utils.GetClusterID`. |
| `internal/pipeline/handlers.go` | `RegisterInformer.publishItem` changes its terminal call from `ri.outputWriter.Write(k8sResource, evtype, config)` to invoking the adapter-supplied `EmitFunc` (threaded in via a new field on `RegisterInformer`, populated by `internal/discovery/kubernetes`). Behavior-preserving; only the seam moves. |
| `pkg/model/resource.go` (new) | `model.Resource` - see 5.1. `KubernetesResource` is unchanged; `Resource` wraps/embeds it for the K8s case and is the new emit-time envelope. |
| `pkg/utils/utils.go` | No change to `GetClusterID`'s implementation; add a comment cross-referencing `discovery.Discoverer.ScopeID` as its new caller path, so a future reader isn't surprised to find two names for one concept. |
| `pkg/lib/meshsync/options.go` | Add `Options.Platform string` (default `"kubernetes"`, preserving today's implicit behavior) and `WithPlatform(string) OptionsSetter`. Extend `AllowedOutputModes`-style validation with an `AllowedPlatforms` list gated behind the Phase 1 feature flag (5.6 / 6). |
| `pkg/lib/meshsync/meshsync.go` | `Run(...)` resolves a `Discoverer` via the new `discovery.Registry` keyed by `options.Platform` instead of hardcoding `mesherykube.New` + `meshsync.New`; the Kubernetes path is refactored to go through the registry too (dogfooding the abstraction immediately, so there is only one code path to maintain, not "K8s special-cased + new interface unused"). |
| `internal/output/output.go` | `Writer.Write`'s first parameter type changes from `model.KubernetesResource` to `model.Resource` (5.1 makes this a strict superset via embedding, so this is source-compatible for every existing K8s-only call site once `model.Resource.KubernetesResource` is populated - see 7 for the precise compatibility shim). |
| `internal/discovery/docker/docker.go` (Phase 1, new) | Second concrete `Discoverer`. See 4.2. |
| `internal/config/types.go` | No structural change; `PipelineConfig.Name` keeps its GVR-string shape for Kubernetes only - Docker's adapter defines its own internal resource-kind enum, not reusing `PipelineConfig`. |
| `docs/agent-instructions/architecture.md` | New "Discovery Abstraction" section documenting the `Discoverer` interface and adapter list, superseding the current all-Kubernetes description; cross-link from the design-spec docs. |
| `docs/design-spec_meshsync-infrastructure-synchronization.md` | Update the "Object Model" section (currently poses the abstract-vs-K8s-native question as an open question) to record the decision made in Section 5 below, so the design doc stops describing this as unresolved. |

### 4.2 Docker adapter shape (Phase 1 MVP - see Section 10 for why Docker, not cloud, is first)

- `internal/discovery/docker/docker.go`: uses `github.com/docker/docker/client` (new dependency) against the Docker Engine API - either the local Unix socket (embedded/out-of-cluster mode, matching how the embedded MeshSync library already runs in-process) or a remote `DOCKER_HOST` (TCP+TLS).
- `Discover`: lists containers, images, networks, volumes (`client.ContainerList`, `ImageList`, `NetworkList`, `VolumeList`).
- `Watch`: subscribes to the Docker Events API (`client.Events`) filtered to `container`/`network`/`volume`/`image` types, translating each event into an Add/Update/Delete emit - this is Docker's informer-equivalent (push-based, not poll), which keeps the adapter's operational profile close to the Kubernetes one.
- `ScopeID`: the Docker daemon's `client.Info().ID` (a stable per-daemon UUID Docker already generates) - the direct Docker analog of `kube-system`'s namespace UID.
- Resource kind mapping: `Container -> Kind:"Container"`, `Image -> Kind:"Image"`, `Network -> Kind:"Network"`, `Volume -> Kind:"Volume"` (mirroring Kubernetes' `Kind` field usage so Server-side code that switches on `Kind` needs the smallest possible extension).

### Meshery Server

| File | Change |
|---|---|
| `models/meshsync_events.go` | `MeshsyncDataHandler.Unmarshal` / `persistStoreUpdate` / `meshsyncEventsAccumulator` widen from `meshsyncmodel.KubernetesResource` to `meshsyncmodel.Resource` (5.1's new MeshSync type). Since `Resource` embeds `KubernetesResource`, every existing field access (`obj.KubernetesResourceMeta.Name`, `obj.Kind`, etc.) keeps compiling unchanged for the Kubernetes path. |
| `machines/helpers/auto_register.go` | `getTypeOfConnection` (line 180) extended to also fingerprint off `data.Obj.Platform` (e.g. a Docker container image named `grafana/grafana` or `prom/prometheus` maps to `"grafana"`/`"prometheus"` connection kind directly, no name-substring heuristic needed for the container case). `ComponentMetadata`/`capabilities` annotation (currently produced only by the Kubernetes `GetProcessorInstance` path in MeshSync, `pkg/model/model_converter.go:99`) needs a Docker-side equivalent producing the same `capabilities.connection`/`capabilities.urls` shape - see 4-repo dependency in Section 6. |
| `machines/docker/` (new package, Phase 1+) | Mirrors `machines/kubernetes/`'s pattern: `machine.go` (states -> `DiscoverAction`/`RegisterAction`/`ConnectAction`/... structs), wired into `machines/helpers/helpers.go:37`'s `getMachine` switch as `case "docker":`. Minimal viable version can reuse `machines.DefaultConnectAction` (as `grafana`/`prometheus` already do) rather than writing bespoke actions for every state on day one. |
| `handlers/` (new handler, or extend `connections_handlers.go`) | A registration entry point analogous to `k8sconfig_handler.go`'s `addK8SConfig` - e.g. `POST /api/integrations/connections/docker` accepting a Docker host address (+ optional TLS cert bundle as credential secret), creating a `docker` `Connection` in `discovered`/`connected` state and (Phase 1.5) triggering embedded MeshSync-Docker in-process against that host (see 6.4 on why standalone-daemon deployment is deferred). |
| `internal/graphql/` | No schema change needed for the generic Connection type (already kind-agnostic in the GraphQL layer per `internal/graphql/schema/schema.graphql` modeling `Connection` generically) - verify field coverage during Phase 1 implementation, not part of this design's blocking dependency. |

### meshery/schemas

| File | Change |
|---|---|
| `schemas/constructs/v1beta3/resource/resource.yaml` (new construct family) | The platform-agnostic **discovered-resource** schema (5.1). This is new - there is no existing `resource` construct family; create `models/v*/resource/` alongside `connection/`, `component/`. |
| `schemas/constructs/v1beta3/connection/connection.yaml` | No structural change to `Connection` itself (its `Kind`/`Type`/`SubType`/`Metadata` already accommodate `docker`/`aws` values as free strings). Add non-normative documentation (description text) enumerating `docker`, `aws`, `gcp`, `azure` as now-populated `Kind` values, since the field's description already lists example kinds inline. |
| `schemas/schemas/constructs/v1beta1/connection/connections/docker.model.yaml` (new, populates the previously-unused `ConnectionDefinition`/`connections/` folder mechanism) | First real `ConnectionDefinition` for `kind: docker`: `type: platform`, `subType: orchestration`, `credentialSchema` (host + optional TLS client cert/key), `connectionSchema` (host, API version), `transitionMap` mirroring the existing generic FSM states. |
| `validation/consumer_audit.go` / `make consumer-audit` | Run against both new schemas (per CLAUDE.md's "Schema-Driven Implementation" and the repo's own required-on-every-PR item) before merging any consumer PR that reads the new `resource` construct. |

### Meshery Operator

No code change in Phase 0/1. Operator remains Kubernetes-only by design (Section 4d/6.4); it is out of scope for Docker/cloud deployment. Document this explicitly in `meshery-operator/docs/architecture.md` (one paragraph: "Operator deploys MeshSync only for in-cluster Kubernetes discovery; non-Kubernetes discovery runs embedded in Meshery Server or as a standalone process outside Operator's remit") so the boundary is not accidentally assumed away by a future reader of that repo.

### MeshKit

No structural change required for Phase 0/1. `broker.Message.Object interface{}` already accepts `model.Resource` without modification. If Phase 2 (cloud) needs shared cloud-SDK credential handling (STS/IAM assume-role helpers reused by multiple Meshery components beyond MeshSync), that utility belongs in MeshKit per CLAUDE.md's ecosystem rule ("shared utils... belong in MeshKit") - flagged as a Phase 2+ dependency, not designed here.

## 5. Schema/Model Changes

### 5.1 The platform-agnostic resource model - reconciling three constraints

Three requirements pull in different directions and must be resolved explicitly:

1. **Schema-first rule**: a platform-agnostic resource model must originate in `meshery/schemas`.
2. **Zero-regression rule** (CLAUDE.md naming-conventions.md): MeshSync's `KubernetesResource` wire shape cannot be silently recast; Server's DB schema and every persisted snapshot depend on it verbatim today.
3. **Incremental-only rule** (this task's explicit instruction): no big-bang migration of `pkg/model` into schemas in this pass.

**Decision: introduce a schemas-defined `Resource` envelope now; do not migrate `KubernetesResource` into schemas in this phase.**

```yaml
# schemas/schemas/constructs/v1beta1/resource/resource.yaml (new)
$id: https://schemas.meshery.io/resource.yaml
type: object
required: [id, platform, kind, schemaVersion]
properties:
  id: { $ref: ../core/api.yml#/components/schemas/Uuid }
  platform:
    type: string
    description: Discovery platform this resource was observed on (kubernetes, docker, aws, azure, gcp)
    maxLength: 64
  scopeId:
    type: string
    description: Identity of the platform-specific scope this resource belongs to (Kubernetes cluster UID, Docker daemon ID, cloud account+region) - the platform-agnostic generalization of MeshSync's historical clusterId field.
  kind: { type: string, description: Resource kind within its platform (Pod, Container, EC2Instance, ...) }
  name: { type: string }
  labels:
    type: object
    x-go-type: core.Map
    description: Platform-native labels/tags/annotations, normalized to a flat string-keyed map (Kubernetes labels+annotations, Docker container labels, cloud resource tags all fit this shape).
  metadata:
    type: object
    x-go-type: core.Map
    description: Platform-specific payload not otherwise modeled (mirrors ConnectionDefinition's pattern of carrying the kind-specific shape in an open metadata bag).
  connectionId:
    $ref: ../core/api.yml#/components/schemas/Uuid
    description: Connection this resource belongs to, once one exists (nullable - a resource can be discovered before its owning connection is registered).
  schemaVersion: { $ref: ../core/api.yml#/components/schemas/VersionString }
```

Go side (in `meshsync/pkg/model`, **not regenerated from schemas yet** - see rationale below):

```go
// pkg/model/resource.go (new)
package model

import resourcev1beta1 "github.com/meshery/schemas/models/v1beta1/resource"

// Resource is the emit-time envelope every Discoverer produces. It carries
// the schemas-defined platform-agnostic fields plus, for the Kubernetes
// platform specifically, the full legacy KubernetesResource so every existing
// Server-side field access keeps compiling unchanged.
type Resource struct {
    resourcev1beta1.Resource
    // Kubernetes is populated only when Platform == "kubernetes"; nil for
    // every other platform. This embedding IS the back-compat shim (see
    // Section 7): existing code paths that type-assert or dereference
    // KubernetesResource-shaped fields keep working for the K8s case, and
    // the broker/DB path can widen field-by-field over subsequent PRs
    // instead of in one migration.
    Kubernetes *KubernetesResource `json:"kubernetes,omitempty"`
}
```

**Why not generate `Resource` from schemas as MeshSync's actual Go struct immediately:** MeshSync's build/errorutil/GORM tooling is tightly coupled to hand-written structs today (`BeforeCreate`/`BeforeSave` hooks in `pkg/model/model.go:71`, custom `gorm` tags throughout). Forcing an oapi-codegen-generated type through that machinery in the same change that also introduces the `Discoverer` interface is exactly the kind of scope-compounding this task's "incremental" instruction warns against. The schemas YAML is authored now (satisfying schema-first), a hand-written Go struct in MeshSync mirrors it field-for-field now, and the codegen swap (replacing the hand-written struct with the generated one) is a explicitly separate, later migration - tracked as an open dependency in Section 11, not silently deferred.

### 5.2 `scopeId` vs `clusterId` - the compatibility decision

`KubernetesResource.ClusterID` (`json:"cluster_id"`) is load-bearing on the wire (Server persists and queries by it) and is explicitly flagged by MeshSync's own naming-conventions.md as **must-not-silently-recase**. Resolution:

- `model.Resource.ScopeID` is the new, platform-agnostic field, populated by every `Discoverer.ScopeID()`.
- For the Kubernetes platform specifically, `Resource.Kubernetes.ClusterID` continues to be set (unchanged field, unchanged tag) **and** `Resource.ScopeID` is set to the identical value. This is intentional duplication for one migration window, not an oversight - it lets Server-side consumers migrate their queries from `cluster_id` to `scope_id` at their own pace (a column addition + backfill, not a rename) before `cluster_id` is ever deprecated. Document the dual-write explicitly in a code comment at the write site so a future maintainer does not "clean up" the duplication prematurely.

### 5.3 Reconciling with Server's Connection/component model

No schema change is needed to `Connection` itself (Section 4/schemas row above) - it already accommodates non-K8s kinds structurally. The reconciliation work is entirely in the **discovered-resource -> Connection promotion path** (`AutoRegistrationHelper`, Section 4/Server rows), which is Server-side application logic, not a schema gap.

### 5.4 Versioning

- New `resource` construct family starts at `v1beta1` (matching schemas' current in-flight major line, not `v1alpha1`, since this is a deliberate net-new construct being added to an already-stabilizing schema set, mirroring how `connection` itself entered at `v1beta1` per the existing directory structure).
- `Resource.SchemaVersion` follows the same `connections.meshery.io/v1beta2`-style default-value pattern already established on `Connection.schemaVersion` (`connection.yaml:140`).
- No breaking change to any existing schemas construct; this is purely additive.

## 6. Cross-Repo Sequencing & Feature-Flagging

**Refactor-first phasing, hard sequencing (each step is a mergeable, independently-shippable unit):**

1. **MeshSync Phase 0a** (this repo, no external dependency): add `internal/discovery` package + `Discoverer` interface + `kubernetes` adapter wrapping existing `meshsync.Handler`/`internal/pipeline` unchanged. Add `pkg/model.Resource` as a MeshSync-local struct (schemas construct not yet required for this step - ship it hand-written first, or in lockstep if schemas PR is ready). `internal/output.Writer.Write`'s signature widens to `model.Resource`. **This step alone changes zero observable behavior for existing Kubernetes deployments** - it is pure internal restructuring, verified via the `verifier-meshsync` skill against a real cluster to confirm no regression (Section 9).
2. **schemas PR** (parallel to step 1, or immediately after): author `resource.yaml` construct, generate Go/TS bindings, run `make validate-schemas && make consumer-audit`. Land independently; MeshSync step 1 references the generated `resourcev1beta1.Resource` type once merged (or ships with the hand-written mirror first and swaps the import in a fast-follow PR - either ordering is safe since the shapes are identical by construction).
3. **MeshSync Phase 1**: `internal/discovery/docker` adapter + `pkg/lib/meshsync.Options.Platform` + `WithPlatform`. Gate behind an explicit opt-in (`Platform: "docker"` must be requested; default stays `"kubernetes"`) - this is the feature flag: no existing deployment is affected unless it explicitly asks for Docker discovery.
4. **Server PR(s)**: widen `meshsyncmodel.KubernetesResource` -> `meshsyncmodel.Resource` in `models/meshsync_events.go` and `machines/helpers/auto_register.go` (source-compatible per the embedding in 5.1, so this can land *before* the Docker adapter exists and before any Docker connection kind is registered - it only changes the declared type, not behavior, since `Resource.Kubernetes` is always populated on the only platform that currently emits anything). Add `machines/docker/` package + `getMachine` case. Add the Docker registration handler.
5. **Server + schemas**: populate the first real `ConnectionDefinition` for `kind: docker` (schemas PR from Section 4) and wire `AutoRegistrationHelper.getTypeOfConnection`'s Docker-aware fingerprinting.
6. **Meshery Operator**: no change; explicitly document the boundary (Section 4).

**Feature flag placement:** `Options.Platform` in MeshSync (compile-time-safe, runtime-selected) is the primary flag. Server-side, the Docker registration handler and `machines/docker` package are dead code until a user calls the new endpoint - no runtime flag needed there beyond the endpoint's own existence, consistent with how `grafana`/`prometheus` machines were added previously (no flag, just new code paths nobody hits until invoked).

## 7. Back-Compat & Migration

- **Wire format**: `internal/output.BrokerWriter.Write` still calls `br.Publish(config.PublishTo, &broker.Message{ObjectType: broker.MeshSync, EventType: evtype, Object: obj})` - `obj`'s Go type changes from `model.KubernetesResource` to `model.Resource`, but since `Resource` embeds `KubernetesResource` as a named field (`Kubernetes *KubernetesResource`), the **JSON shape is not identical** - existing consumers doing `json.Unmarshal(msg, &meshsyncmodel.KubernetesResource{})` directly on a `Resource`-shaped payload would silently get zero-valued top-level fields today addressed via the embedded pointer. This is the one real breaking-wire-shape risk in this design, and it must be handled as follows: **Server's `Unmarshal` (`models/meshsync_events.go:197`) is the only production consumer and is updated in lockstep (Section 6 step 4)** - there is no third-party or historical consumer of this exact broker subject outside the Meshery monorepo (confirmed: `MeshsyncStoreUpdatesSubject`/`meshsync-data` ObjectType are internal to `meshery/meshery`). Snapshot **files** written by `-output file` mode are a different concern: a file written by an older MeshSync binary and later read by code expecting the new `Resource` shape would fail to populate `.kubernetes.*` - flag this explicitly to file-mode consumers (e.g. `mesheryctl`'s local import path, if any) as a one-time format bump; confirm during implementation whether any tooling parses snapshot files by shape rather than by re-running the same MeshSync binary version.
- **DB compatibility**: Server's `dbHandler.Model(&meshsyncmodel.KubernetesResource{})` calls (e.g. `auto_register.go:127`) are unaffected as long as the DB table/GORM model itself (`pkg/model.KubernetesResource`) is not renamed or restructured - it is not, in this design; only a new sibling field/type wraps it.
- **`cluster_id` migration**: per 5.2, no rename occurs in this phase; `scopeId` is additive.
- **Meshery Operator / CRD**: `MeshSyncSpec` is untouched; existing CRs continue to work with zero change, since the platform selection lives in MeshSync's own `Options`/CLI flag, not in the Operator-managed CR (Operator only ever deploys the Kubernetes platform anyway - Section 6.6).
- **`pkg/lib/meshsync.Options`**: `Platform` field defaults to `"kubernetes"`; every existing embedder of the library (today: none in `meshery/server` per Section 2/Server findings, but any external consumer of the library) continues to get identical behavior with zero code change.

## 8. Risks / Failure Modes & Effort Honesty

This is explicitly the largest, most speculative item in the roadmap. Named risks:

- **"Informer" has no Docker/cloud equivalent with the same semantics.** Kubernetes' watch+resync model (list-then-watch, resourceVersion-based resume, cache-store-backed) has no drop-in analog: Docker's Events API is push-only with no resume-from-checkpoint guarantee across a restart (a MeshSync restart could miss events during the gap and must reconcile via a fresh `Discover` pass on start - functionally fine but must be designed into `Watch`'s contract, not bolted on later); most cloud inventory APIs (AWS Config/Resource Groups Tagging API, etc.) have **no push mechanism at all** and require poll-diff, which is a fundamentally different reconciliation model from everything else in this pipeline. The `Discoverer.Watch` contract in 3.1 is written to tolerate this (poll-diff is a valid `Watch` implementation), but expect the cloud adapter's effort to be dominated by getting poll-diff correctness/performance right, not by API-call plumbing.
- **Rate limits and cost.** Cloud inventory APIs are rate-limited and, for some providers, metered per call; a naive poll loop at Kubernetes-informer cadence would be both throttled and expensive. This pushes the cloud adapter toward a much coarser poll interval (minutes, not the near-real-time Kubernetes gets), which changes the user-facing latency expectation and must be called out in any cloud-facing UI/docs work, not silently absorbed.
- **Credential model divergence.** Kubernetes connections use a kubeconfig/service-account token; Docker uses host+optional mTLS client cert; AWS/GCP/Azure each have distinct credential shapes (IAM role/access key, service-account JSON, service-principal secret). `ConnectionDefinition.credentialSchema` (schemas) is built for exactly this per-kind variance, but each cloud's credential UX (e.g., federated/assume-role flows) is itself a multi-week UI+backend effort per provider - do not underestimate this as "just another credential form."
- **Fingerprinting/auto-registration heuristic is currently a toy** (`getTypeOfConnection`'s substring match, `auto_register.go:180`). Extending it per-platform (Section 4/Server) is a real design surface of its own (how does MeshSync-Docker know a container labeled `grafana/grafana:latest` should offer a `grafana` connection versus staying a bare `docker` container resource?) - this design proposes the mechanical extension point but does not solve fingerprinting robustness; expect iteration.
- **Resource-model granularity mismatch.** Kubernetes' `Kind` taxonomy (Pod, Deployment, ConfigMap, ...) and Docker's (Container, Image, Network, Volume) and AWS's (thousands of resource types across services) are not remotely the same size or shape. `model.Resource.Kind` as a free string accommodates this, but any UI/topology code that currently assumes "Kind" implies a small, closed, Kubernetes-shaped set (if any exists in Meshery's Kanvas visualization layer) will need its own audit - **not scoped or verified in this blueprint**; flagged as an open dependency (Section 11).
- **Effort honesty**: Phase 0 (interface refactor) is a well-bounded, low-risk, single-repo change (days, not weeks, given the wrapper-not-rewrite approach). Phase 1 (Docker MVP end-to-end, including Server-side registration + auto-promotion + minimal UI verification) is a multi-week, multi-repo effort touching four of the five repos in this brief. Cloud (Phase 2+) is materially larger again, is fundamentally a **per-provider** effort (AWS != Azure != GCP in credential model, API shape, rate-limit profile, and poll-diff correctness), and should not be estimated as a single unit - budget it as N independent projects, one per cloud, each roughly Docker-MVP-sized or larger.

## 9. Test Plan (+ runtime verification)

**MeshSync unit/integration (`make test`, `make integration-tests`):**
- `internal/discovery/kubernetes`: table-driven test asserting `Discoverer.Discover`/`Watch` produce identical `model.Resource.Kubernetes` output to today's direct pipeline path for a fixed set of fixture objects (regression guard for the wrapper refactor).
- `internal/output`: existing `Writer` tests updated to construct `model.Resource{Kubernetes: &KubernetesResource{...}}` instead of a bare `KubernetesResource` - confirms the widened signature is a strict behavioral no-op for the K8s-only case.
- `internal/discovery/docker` (Phase 1): unit tests against the Docker Go client's documented fake/mock transport (or a `dockertest`-style ephemeral container) for `Discover` (list) and `Watch` (events) correctness, plus a `ScopeID` stability test (same daemon, two calls, same ID).
- `pkg/model.Resource`: JSON round-trip test confirming `Resource{Kubernetes: ...}` marshals/unmarshals losslessly and that `ScopeID`/`ClusterID` dual-write (5.2) is present on every Kubernetes-platform resource.

**Runtime verification (`verifier-meshsync` skill, per this repo's required workflow):**
- Phase 0: run the skill's existing quick-start (`up` / `run` / drive events / observe log lines) **unchanged**, to confirm the `Discoverer`-wrapped Kubernetes path produces byte-identical log signal lines (`Received ADD event for`, resync signals, etc.) and snapshot file content to pre-refactor MeshSync - this is the regression gate for "the wrapper changed nothing observable."
- Phase 1 (Docker): extend the skill (or add a sibling `verifier-meshsync-docker` skill/script) to: start MeshSync with `Platform: docker` in file-output mode against a local Docker daemon, `docker run`/`docker stop` a container, and confirm an Add/Delete signal line appears and the snapshot file contains a `platform: docker` resource - mirroring the existing skill's "drive the surface, read the log, reconcile counts" methodology exactly.

**Server integration:**
- `server/integration-tests/meshsync/integration_test.go` and `database_content_assertion_testcases_test.go` extended with a fixture asserting the widened `meshsyncmodel.Resource` type persists and round-trips through `MeshsyncDataHandler` identically to today for Kubernetes payloads (no fixture changes needed for existing K8s test data, since `Resource.Kubernetes` mirrors `KubernetesResource` field-for-field).
- New integration test (Phase 1) exercising the Docker registration handler end-to-end: register a Docker connection, confirm a `docker` `Connection` row appears in `discovered`/`connected` state via the generic `machines.StateMachine`.

**schemas:**
- `cd ../schemas && make validate-schemas && make consumer-audit` before any MeshSync/Server PR that consumes the new `resource` construct (per this repo's own CLAUDE.md required-on-every-PR item).

## 10. Effort & Phasing

**Phase 0 - K8s-discoverer-behind-interface refactor (MeshSync only, ~days):**
- [ ] `internal/discovery/discoverer.go`: `Discoverer` interface + `EmitFunc` + `Registry`.
- [ ] `internal/discovery/kubernetes/kubernetes.go`: wraps existing `meshsync.Handler`/`internal/pipeline` unchanged.
- [ ] `pkg/model/resource.go`: `Resource` struct embedding `KubernetesResource` (schemas-mirrored shape, hand-written for now per 5.1's rationale).
- [ ] `internal/output/output.go` + all `Writer` implementations: widen signature to `model.Resource`.
- [ ] `internal/pipeline/handlers.go`: `publishItem` calls the injected `EmitFunc` instead of `outputWriter.Write` directly.
- [ ] `pkg/lib/meshsync/options.go` + `meshsync.go`: `Options.Platform` (default `kubernetes`), `Run` resolves via `discovery.Registry`.
- [ ] Runtime-verify via `verifier-meshsync` skill: confirm zero observable regression.
- [ ] Update `docs/agent-instructions/architecture.md` + `docs/design-spec_meshsync-infrastructure-synchronization.md`.

**Phase 1 - First non-K8s MVP: Docker (multi-repo, weeks):**
- [ ] schemas: author + generate `resource` construct (v1beta1); `make validate-schemas && make consumer-audit`.
- [ ] MeshSync: `internal/discovery/docker/docker.go` (Discover/Watch/ScopeID over Docker Engine API).
- [ ] MeshSync: `WithPlatform("docker")` wired through `pkg/lib/meshsync`; unit + `dockertest`-based integration tests.
- [ ] Server: widen `meshsyncmodel.KubernetesResource` -> `meshsyncmodel.Resource` in `models/meshsync_events.go`, `machines/helpers/auto_register.go`.
- [ ] Server: `machines/docker/` package + `getMachine` case (minimal viable: reuse `machines.DefaultConnectAction` as `grafana`/`prometheus` do).
- [ ] Server: Docker connection registration handler (host + optional TLS credential).
- [ ] schemas: first populated `ConnectionDefinition` for `kind: docker`.
- [ ] Server: extend `AutoRegistrationHelper.getTypeOfConnection` for Docker-container-image fingerprinting.
- [ ] Runtime-verify Docker adapter (extend/clone `verifier-meshsync` skill methodology).
- [ ] Docs: docs.meshery.io update for Docker connection registration (external, user-facing, per this repo's CLAUDE.md required-on-every-PR rule).

**Phase 2+ - Cloud (per-provider, materially larger, not detailed here):**
- Read-only cloud-inventory poller as the `Discoverer` shape for AWS first (largest existing Meshery cloud-model footprint, per `meshery/models` icon assets already present) - poll-diff `Watch` implementation, coarse polling interval, IAM-role credential model via `ConnectionDefinition.credentialSchema`.
- Explicitly budget each additional cloud provider as an independent project of comparable or greater size to the Docker MVP (Section 8) - do not batch AWS/Azure/GCP into one estimate.

## 11. Open Questions & Hard Dependencies

- **Does any tooling parse MeshSync snapshot files by shape rather than by re-running the matching MeshSync binary version?** If `mesheryctl` or another consumer reads `-output file` snapshots independently, the `Resource` wrapper's JSON-shape change (Section 7) needs a documented format-version bump communicated to that consumer - not confirmed one way or the other in this pass; must be checked before Phase 0 ships.
- **Kanvas/topology visualization's assumption about `Kind`.** Not audited in this blueprint - confirm whether any UI code hardcodes a closed Kubernetes-`Kind` enum (as opposed to treating it as an opaque string) before Docker/cloud resources reach the UI layer, or non-K8s resources may render incorrectly or not at all despite being correctly persisted.
- **Fingerprinting robustness** (Section 8) is a real open design problem, not solved here - the mechanical extension point is proposed, the heuristic itself is explicitly punted.
- **Whether `pkg/model.Resource` should eventually be generated from schemas rather than hand-mirrored** (Section 5.1's deferred codegen swap) is a genuine follow-up decision, not a foregone conclusion - it depends on how invasive making `KubernetesResource`'s GORM hooks/tags compatible with oapi-codegen output proves to be, which is unknown until attempted.
- **Standalone (non-embedded) Docker/cloud MeshSync deployment topology** (daemon-set-like, always-on process outside both Operator and Server) is named in the prompt's item (d) but not designed here in depth: this blueprint's Phase 1 assumes **embedded-in-Server** (matching the existing `meshsync_deployment_mode` precedent and the fact that `pkg/lib/meshsync` already runs in-process), deferring a standalone-process deployment model as a later decision once real usage patterns for Docker/cloud discovery are observed - flagged honestly as scoped out, not silently assumed unnecessary.
- **Hard dependency**: Phase 1's Server-side changes cannot land functionally complete before the schemas `resource` construct PR merges (Section 6 sequencing) - the two repos must coordinate the PR order or accept a temporary hand-written-only period in MeshSync per 5.1's fallback.
- **Hard dependency**: any UI work surfacing Docker/cloud connections in Meshery's Connections page depends on confirming the GraphQL/REST layer's Connection projection is already kind-agnostic end-to-end (Section 4/Server notes this as "verify during Phase 1 implementation, not blocking this design") - if it is not, that is an additional undiscovered Phase 1 task.

---

### Key files referenced (absolute paths)

- `meshsync/meshsync.go`, `internal/pipeline/pipeline.go`, `internal/pipeline/step.go`, `internal/pipeline/handlers.go`
- `pkg/model/model.go`, `pkg/model/model_converter.go`
- `internal/output/output.go`, `internal/output/broker.go`, `internal/output/composite.go`, `internal/output/processor.go`
- `pkg/lib/meshsync/meshsync.go`, `pkg/lib/meshsync/options.go`
- `pkg/utils/utils.go`, `internal/config/types.go`, `internal/config/default_config.go`
- `docs/agent-instructions/architecture.md`, `docs/design-spec_embedded-meshsync.md`, `docs/design-spec_meshsync-infrastructure-synchronization.md`
- `.claude/skills/verifier-meshsync/SKILL.md`
- `meshery/server/models/connections/connections.go`, `meshery/server/models/connections/meshsync_deployment_mode.go`
- `meshery/server/machines/state.go`, `meshery/server/machines/models.go`, `meshery/server/machines/helpers/helpers.go`, `meshery/server/machines/helpers/auto_register.go`
- `meshery/server/machines/kubernetes/machine.go`, `meshery/server/machines/kubernetes/discover.go`
- `meshery/server/models/meshsync_events.go`, `meshery/server/handlers/k8sconfig_handler.go`
- `schemas/schemas/constructs/v1beta2/connection/connection.yaml`, `schemas/models/v1beta2/connection/connection.go`
- `schemas/schemas/constructs/v1beta3/connection/api.yml`
- `schemas/models/v1beta1/connection/connection_helper.go`, `schemas/models/v1beta1/connection/meshsync_deployment_mode_test.go`
- `meshery-operator/api/v1alpha1/meshsync_types.go`, `meshery-operator/pkg/meshsync/meshsync.go`
- `meshkit/broker/messaging.go`, `meshkit/orchestration/design.go`
