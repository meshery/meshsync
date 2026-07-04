# IMPLEMENTATION BLUEPRINT: Tiered & Configurable Discovery

## 1. Goal & Gap

**Goal.** Replace MeshSync's flat, all-or-nothing discovery model with a three-tier funnel — Tier 1 (cheap/broad presence detection), Tier 2 (deeper per-resource inspection), Tier 3 (infrastructure-specific deep discovery, conditionally activated) — that is user-configurable per connection/cluster for depth and scope, while preserving the existing whitelist/blacklist as a degenerate/back-compat case.

**Gap, precisely.** Today:
- `internal/config/default_config.go:19-370` hardcodes ~66 resource kinds into exactly two buckets, `GlobalResourceKey` and `LocalResourceKey` — there is no third bucket and no notion of "tier."
- `internal/config/crd_config.go:126-135` (`validateLists`) hard-requires exactly one of whitelist XOR blacklist. Both are flat resource-name lists; neither carries a depth/scope concept.
- `internal/pipeline/pipeline.go:15-77` (`pipeline.New`) builds exactly two discovery stages (global, local) plus `StartInformers`. There is no conditional third stage and no mechanism to spin one up only when an infrastructure is detected.
- The MeshSync CRD (`meshery-operator/api/v1alpha1/meshsync_types.go:39-46` and its byte-identical `v1alpha2` sibling) exposes `WatchList corev1.ConfigMap` — an unstructured ConfigMap, not a typed depth/scope contract — plus `Broker`, `Version`, `Size`. No depth/tier field exists in either API version.
- Even the embedded/no-CRD path (`internal/config/config_local.go`) already smuggles infra-specific CRDs (`grafanas.v1beta1.grafana.integreatly.org`, `prometheuses.v1.monitoring.coreos.com`) into the flat whitelist — proof that "infra-specific conditional discovery" is already a felt need, done ad hoc, with no gating on whether that infrastructure is actually present.
- **No live-reload path exists.** `GetMeshsyncCRDConfigs` (`pkg/lib/meshsync/meshsync.go:63`) runs exactly once, at process start, inside `Run`. The only two things that mutate `config.ResourcesKey` after startup are (a) `updatePipelineConfig` (`meshsync/handlers.go:474-528`), which patches in/out single resources when a **CRD definition** (not the MeshSync CR) appears/disappears, and (b) nothing else. The `ReSyncDiscoveryEntity` broker message (`meshsync/handlers.go:256-259`) only re-runs `pipeline.New` against whatever is *already* in `cfg` — it never re-reads the MeshSync CR. **Editing the CR's `watch-list` today requires a pod restart to take effect.** This is a load-bearing fact for the design below: a tier/depth config that lives in the CR needs its own watch-and-reload path, not a ride on the existing `ReSync` channel.
- Design intent already exists and is precise: `docs/design-spec_meshsync-infrastructure-synchronization.md` §"Tiered Discovery" (lines 229-235) and §"MeshSync Discovery Funnel" (lines 329-394) lay out exactly this: Stage 1 Global, Stage 2 Namespace, Stage 3 "Mesh discovery" where "Each step will spin its own pipeline only if the [infrastructure] exists" and "Adapters will be the subscribers of NATS for the data generated from this stage." This blueprint operationalizes that spec against the current codebase rather than inventing a new model.

## 2. Current State Per Repo (cited)

**MeshSync**
- Two hardcoded pipelines: `Pipelines[GlobalResourceKey]`, `Pipelines[LocalResourceKey]` (`internal/config/default_config.go:19-369`).
- `PopulateConfigsFromMap` (`internal/config/crd_config.go:90-114`) requires exactly one of whitelist/blacklist non-empty (`validateLists`, lines 126-135); whitelist entries are `ResourceConfig{Resource string, Events []string}` (`internal/config/types.go:75-78`) — no depth field.
- `pipeline.New` (`internal/pipeline/pipeline.go:15-77`) always builds exactly `gdstage`, `ldstage`, `strtInfmrs` — no conditional stage construction.
- `RegisterInformer.publishItem` (`internal/pipeline/handlers.go:82-112`) is the single per-object write path; `checkMustSkip` (lines 114-133) only filters on namespace today (`outputFiltration.NamespaceSet`), not on any depth/tier concept.
- `meshsync.Handler.Run` (`meshsync/discovery.go` via `meshsync/meshsync.go`, referenced by `docs/agent-instructions/architecture.md:36`) rebuilds the whole pipeline fresh on every resync — the doc explicitly warns against caching stages, which matters for how Tier 3 stages get added/removed.
- `WatchCRDs` (`meshsync/handlers.go:324-458`) watches `apiextensions.k8s.io` CRD *definitions* cluster-wide and incrementally patches `config.ResourcesKey[GlobalResourceKey]` — this is the closest existing analog to "detect infra, then react," but it operates on CRD presence, not Deployment/pod-based infra detection (e.g., Istio's control plane has CRDs, but Prometheus-the-Operator's CRD ≠ a running Prometheus instance).

**Meshery Operator**
- `MeshSyncSpec` (`api/v1alpha1/meshsync_types.go:39-46`, identical in `api/v1alpha2/meshsync_types.go:39-46`) has `WatchList corev1.ConfigMap`, `Broker`, `Version`, `Size` — no depth/tier field.
- `MeshSyncReconciler.reconcileMeshsync` (`controllers/meshsync_controller.go:384-402`) Server-Side-Applies only the Deployment; it never re-pushes `Spec.WatchList` contents anywhere at runtime — MeshSync reads the CR directly and only at its own startup.
- The raw CRD OpenAPI schema for `watch-list` (`meshery/install/kubernetes/helm/meshery-operator/crds/crds.yaml:442-611`, both versions) validates it as a generic `corev1.ConfigMap` shape (`apiVersion`, `binaryData`, `data`, etc.) — i.e., completely untyped from the CRD's own validation perspective; all structure is implicit in what MeshSync's Go code expects to find in `Data["whitelist"]`/`Data["blacklist"]`.

**meshery/schemas**
- `models/v1beta1/connection/connection.go` (generated, "DO NOT EDIT") + `connection_helper.go` (hand-written) already establish the exact extension pattern needed: `Connection.Metadata` is a free-form `core.Map`; `connection_helper.go` defines a well-known metadata key (`MeshsyncDeploymentModeMetadataKey = "meshsync_deployment_mode"`) plus typed enum + `FromMetadata`/`SetXToMetadata` helper functions, consumed via `github.com/meshery/schemas/models/v1beta1/connection` throughout `meshery/server`. This is a precedent for "typed sub-contract nested in `Connection.Metadata`, defined once in schemas, consumed everywhere" — the same shape I'll use for discovery config, rather than growing the core `Connection` entity schema itself.
- No `constructs/v1beta1/meshsync` or `discovery` directory/schema exists yet.

**Meshery Server**
- Subscribes on `meshery.meshsync.core` (`models/meshsync_events.go:127`, mirrored `internal/graphql/model/operator_helper.go:28`) and treats every message as a FULL object create/update/delete into GORM (`meshsyncEventsAccumulator`, `models/meshsync_events.go:209-252`) — this is the wire contract that must not silently break.
- `MesheryDataHandler.Resync()` (`models/meshsync_events.go:378-392`) publishes `MeshsyncRequestSubject` (`meshery.meshsync.request`) with `Entity: broker.ReSyncDiscoveryEntity` — already wired end-to-end: `MesheryControllersHelper.ResyncMeshsync` (`models/meshery_controllers.go:310-315`) → the Kubernetes connection state machine's `ResyncResources` handler (`machines/kubernetes/resync_resources.go`) is a state-machine-triggerable action today.
- `UpdateConnectionById` (`handlers/connections_handlers.go:359-468`) already special-cases `connection.MetaData` for a well-known key (`MeshsyncDeploymentModeFromMetadata`, lines 386-426) and drives a side effect (`handleMeshSyncDeploymentModeChange`) before persisting — this is the established handler-level pattern for "metadata key triggers infrastructure action," which the discovery-config endpoint will mirror.

**Meshery UI**
- `rtk-query/connection.ts:72-82` (`updateConnectionById`) PUTs `{status, metadata}` to `integrations/connections/{connectionId}` and invalidates the `Connection_API_Connections` tag — the exact mutation shape for a new "set discovery config" UI action.
- `ConnectionTable.hooks.ts` (`useConnectionActions`) is the established hook layer wrapping these mutations with notifications — new discovery-depth controls belong here.

**MeshKit**
- `broker/messaging.go:3-39` defines `Message{ObjectType, EventType, Request, Object}` and `RequestObject{Entity RequestEntity, Payload interface{}}` with `RequestEntity` constants (`LogRequestEntity`, `ReSyncDiscoveryEntity`, `ExecRequestEntity`, `ActiveExecEntity`). Adding a Tier-3 "infra detected" notification fits this exact envelope as a new `ObjectType`/`RequestEntity`, not a new transport.

## 3. Proposed Architecture

### 3.1 Tier model (conceptual, mapped onto existing stages)

| Tier | What it does | Maps to |
|---|---|---|
| **Tier 1** | Cheap, broad presence/identity detection: object exists, kind, name, namespace, labels, owner refs. No deep field extraction. | Existing **Global** + **Local** discovery stages, but with a new "shallow" projection mode (see 3.3) |
| **Tier 2** | Deeper per-object inspection for kinds already in Tier 1's scope: images, versions, resource requests/limits, conditions, status detail. | Same Global/Local stages, "full" projection mode (== today's existing full-fidelity behavior, unchanged by default) |
| **Tier 3** | Infrastructure-specific deep discovery, conditionally activated only when that infrastructure is detected present (by Tier 1 evidence: characteristic CRDs and/or namespace/label signatures). Delegated, optionally, to an external adapter/controller subscribing over NATS rather than run in-process. | **New** conditional stage(s) in `internal/pipeline`, gated by an in-process **infrastructure detector** |

Depth is not "tier 1 only" vs "tier 3 only" as separate exclusive modes — it is **cumulative**: Tier 2 requires Tier 1's stage to have run (it inspects the same objects more deeply), and Tier 3 requires Tier 1 to have positively identified the target infrastructure before it activates. This matches the design spec's phased language ("Phase 1... Phase 2... Phase 3") directly.

**Whitelist/blacklist as a degenerate tier config.** The existing whitelist/blacklist becomes tier config with `Tier: unspecified` and `Depth: full` for every listed resource, and Tier 3 disabled. `PopulateConfigsFromMap` continues to accept the old shape byte-for-byte; the new tier-aware shape is additive.

### 3.2 Where "depth" lives without breaking today's model

Depth is **not** a new pipeline stage type — it is a per-resource-kind attribute in `PipelineConfig` (`internal/config/types.go:50-54`) plus a corresponding trim step in the informer→model conversion path (`pkg/model.ParseList`, invoked from `internal/pipeline/handlers.go:93`). Adding `Depth DiscoveryDepth` to `PipelineConfig` lets Tier 1/Tier 2 be expressed as "same informer, same stage, cheaper projection" rather than duplicating stages — this avoids doubling the number of informers (a second informer per kind would double API server watch load, defeating the entire scale goal).

```
                                 +-------------------------------------------+
                                 |     internal/config.DiscoveryConfig        |
                                 |  (new: tiers, depth-per-kind, infra-gates) |
                                 +---------------------+---------------------+
                                                        |
                                     (replaces/extends PopulateConfigsFromMap)
                                                        v
   +---------------- internal/pipeline.New (unchanged signature + 1 param) ----------------+
   |                                                                                        |
   |  Global stage (Tier1/2, existing)   Local stage (Tier1/2, existing)                    |
   |          |                                    |                                        |
   |          +-------------------+----------------+                                        |
   |                              v                                                         |
   |                    StartInformers (existing, unchanged)                                |
   |                                                                                         |
   |  NEW: Infra-detection step (post-sync, reads informer stores already primed)             |
   |                              |                                                          |
   |            detects: characteristic CRDs present? namespace/label signature present?     |
   |                              |                                                          |
   |                 for each DETECTED + ENABLED infra tier-3 target:                         |
   |                              v                                                          |
   |         EITHER (a) in-process Tier-3 stage (new informers scoped to that                 |
   |                  infra's own CRDs/resources, same pipeline mechanics)                     |
   |         OR      (b) publish "infra detected" notification to NATS,                        |
   |                  adapter subscribes and runs its own discovery, publishes                  |
   |                  results back to meshery.meshsync.core itself (adapter is the               |
   |                  producer for its own tier-3 objects; MeshSync does not proxy)              |
   +-----------------------------------------------------------------------------------------+
                                                        |
                                                        v
                                         internal/output.Writer (unchanged)
                                                        |
                                                        v
                                    NATS meshery.meshsync.core (unchanged wire shape)
```

**Why "post-sync step inside the existing pipeline" rather than "new pipeline run after the first pipeline completes."** `internal/pipeline.New` already documents (`docs/agent-instructions/architecture.md:36`) that stages are rebuilt fresh every call and that stale state must never be hoisted. A Tier-3 stage that depends on Tier-1 results (which CRDs exist) cannot be constructed before Tier-1's `StartInformers` step has run and the caches have synced (`internal/pipeline/step.go:100-122`, `WaitForCacheSync`). So Tier-3 stage construction must happen either (a) as a 4th `pipeline.Stage` appended conditionally at the end of the same `pipeline.New` call — but stage *membership* is fixed at construction, before `Run()` — or (b) as a subsequent, separate `pipeline.New(...)`+`Run()` cycle triggered once Tier-1 synced. Given the framework's Stage/Step model has no "decide steps based on the previous stage's output" primitive, **(b) is the correct approach**: after `StartInformers` completes and caches sync, an `InfraDetector` runs against the now-populated `cache.Store`s already available in `internal/pipeline/step.go:58-64` (`data[ri.config.Name] = iclient.Informer().GetStore()`), and for every infra it detects that's both gated-on and not yet running, it starts *a second, independently-lifecycled* pipeline (new stop channel, its own `pipeline.New` call, same mechanics as today's) scoped to that infra's resource kinds. Detected infra-pipelines are tracked in the handler alongside the existing `h.stores` map so a later CR update or CRD-definition change can start/stop them without restarting the primary Tier-1/2 pipeline.

### 3.3 Depth semantics (Tier 1 vs Tier 2 field projection)

`model.KubernetesResource` (`pkg/model`) already separates `KubernetesResourceObjectMeta`/`Spec`/`Status`. Tier 1 ("shallow") publishes `ObjectMeta` only (name, namespace, labels, UID, owner refs, kind/apiVersion) with `Spec`/`Status` empty; Tier 2 ("full") publishes everything, exactly as today. This is implemented as a trim applied to the already-parsed `model.KubernetesResource` right before `outputWriter.Write` in `RegisterInformer.publishItem` (`internal/pipeline/handlers.go:82-112`) — **not** a change to `model.ParseList` itself, so the wire *shape* (struct fields present) is unchanged; only field *values* are zeroed for shallow depth. This preserves the FULL-objects-on-`meshery.meshsync.core` back-compat contract: Server's GORM `Create`/`Updates` calls (`models/meshsync_events.go:224,237`) still receive a well-formed `KubernetesResource`, just with fewer populated fields for shallow-tier kinds — no new message type, no subject change, no consumer code change required for the MVP (Server naturally just persists sparser rows for shallow-depth resources).

### 3.4 Infrastructure detection (Tier 1 → Tier 3 gate)

A new `internal/infra` package defines a small, extensible registry of **infra signatures**:

```go
type InfraSignature struct {
    Name              string   // "istio", "prometheus-operator", ...
    IndicatorCRDs     []string // GVR-ish strings, e.g. "virtualservices.v1beta1.networking.istio.io"
    IndicatorLabels   map[string]string // optional: label selector signature on any Namespace
    Tier3ResourceKeys string   // key into config.Pipelines for that infra's Tier-3 pipeline set, OR
    DelegateToAdapter bool     // true => publish detection event, do not run Tier-3 in-process
}
```

Detection runs by checking the **already-synced Tier-1 informer stores** for the presence of any `IndicatorCRDs` object (cheap: these are CRD *definitions*, discovered the same way `WatchCRDs` already discovers CRDs — reuse `kubernetes.CRDItem`/`GetGVRForCustomResources` from MeshKit's `utils/kubernetes`, already imported in `meshsync/handlers.go:13`). This is O(store lookup), not a new API call.

## 4. Per-Repo Changes

### MeshSync

| File | Change |
|---|---|
| `internal/config/types.go` | Add `DiscoveryDepth` type (`"shallow"`/`"full"`, default `"full"`) and `Tier` type (`1`/`2`/`3`, default unset = legacy). Add `Depth DiscoveryDepth` and `Tier int` fields to `PipelineConfig` (lines 50-54). Add `DiscoveryConfig` struct: `{Mode DiscoveryMode, Depth DiscoveryDepth, EnabledInfra []string, DisabledInfra []string}` nested inside `MeshsyncConfig` (line 67-72) as a new field `Discovery *DiscoveryConfig`. |
| `internal/config/crd_config.go` | New `populateTierConfig(data map[string]string) (*DiscoveryConfig, error)` parsing a new `tier-config` key (JSON) from the CR's `watch-list` ConfigMap `Data`, sibling to today's `whitelist`/`blacklist` keys. `PopulateConfigsFromMap` calls it; if absent, synthesizes `Discovery = &DiscoveryConfig{Mode: ModeWhitelistBlacklist, Depth: DepthFull}` from the existing whitelist/blacklist path unchanged (back-compat: today's `validateLists` requirement of exactly one of white/black stays the default path; tier config makes it optional by supplying a third path). |
| `internal/config/default_config.go` | Add per-`PipelineConfig` `Tier`/`Depth` tags to the existing ~66 entries: Tier 1 for all current Global/Local entries (this is genuinely what they are today — presence + full fidelity, no infra gating), explicitly marking today's behavior rather than silently changing it. |
| `internal/infra/signatures.go` (new) | `InfraSignature` registry (istio, linkerd, prometheus-operator, grafana-operator as initial seed, matching what's already hand-coded into `config_local.go`'s whitelist). |
| `internal/infra/detector.go` (new) | `Detect(stores map[string]cache.Store, signatures []InfraSignature) []DetectedInfra` — pure function over already-synced stores, no new API calls. |
| `internal/infra/error.go` (new) | MeshKit error codes for detection failures, following `docs/agent-instructions/errors.md` convention; allocate from `helpers/component_info.json`'s `next_error_code` (currently `1015`, so next free is `1016`+ — **do not hand-pick specific numbers**, let `errorutil` normalize on merge per `docs/agent-instructions/errors.md:29`). |
| `internal/pipeline/pipeline.go` | `pipeline.New` signature grows one parameter: `depth internalconfig.DiscoveryDepth` (or a full `DiscoveryConfig`), threaded to `newRegisterInformerStep` so it can pass through to `publishItem`. Two call sites to update: `meshsync/discovery.go:34` and any test harness constructing pipelines directly (`internal/pipeline/pipeline_test.go`). |
| `internal/pipeline/step.go` | `RegisterInformer` gains a `depth` field; `Exec` unchanged structurally. |
| `internal/pipeline/handlers.go` | `publishItem` (lines 82-112): after `model.ParseList`, if `ri.depth == DepthShallow`, zero `k8sResource.KubernetesResourceSpec` and `KubernetesResourceStatus` before calling `outputWriter.Write`. This is the single trim point. |
| `meshsync/discovery.go` | `startDiscovery` reads the active `DiscoveryConfig` (via a new `config.DiscoveryKey` alongside existing `config.ResourcesKey`) and passes depth through to `pipeline.New`. After the primary pipeline's `Run()` returns and `h.stores` is populated, call new `h.runInfraDetectionAndTier3(pipelineCh)` (see below). |
| `meshsync/meshsync.go` (Handler struct) | Add `tier3Managers map[string]*tier3Pipeline` (name → {stopChan, stores}) so detected infra-pipelines can be independently torn down on CR change/shutdown, mirroring the existing `h.stores`/`h.informer` lifecycle fields. |
| `meshsync/tier3.go` (new) | `runInfraDetectionAndTier3`: runs `infra.Detect` against `h.stores`; for each newly-detected, enabled, non-delegated infra, builds and runs a scoped `pipeline.New(...)` (its own stop channel) targeting that infra's `config.Pipelines[infraKey]` entries, tracked in `h.tier3Managers`; for each newly-detected, enabled, **delegated** infra, publishes a `broker.Message{ObjectType: broker.InfraDetected, Object: DetectedInfraPayload{...}}` to a new subject `meshery.meshsync.infra-detected` (see MeshKit changes) instead of running discovery itself. Also stops/removes tier3Managers for infra that disappeared or was disabled. |
| `meshsync/handlers.go` | New CR-watch path (see 4-cross-cutting below): `WatchMeshSyncCR()` alongside existing `WatchCRDs()`, watching the single `meshsyncs.meshery.io` CR instance (not all CRD definitions) for `MODIFIED` events on `spec.watch-list`, and on change: re-fetch via `config.GetMeshsyncCRDConfigs`, diff tier/depth/whitelist against the currently active config, and if changed, push new config into `cfg` (`config.DiscoveryKey`/`config.ResourcesKey`) and signal `channels.ReSync` — **this closes the hot-reload gap identified in §1** for the first time, for both the old whitelist/blacklist and the new tier config uniformly. |
| `pkg/lib/meshsync/meshsync.go` | `Run` starts `go meshsyncHandler.WatchMeshSyncCR()` alongside the existing `go meshsyncHandler.WatchCRDs()` (line 222), gated the same way (`if useCRDFlag`). |
| `docs/agent-instructions/architecture.md` | Update the pipeline-stages paragraph (lines 34-39) to describe the conditional Tier-3 stage and the new CR-watch path; this is a required doc update per CLAUDE.md's "documentation is part of the change." |
| `docs/design-spec_meshsync-infrastructure-synchronization.md` | Mark the "Tiered Discovery"/"Discovery Funnel" sections as **implemented** with a pointer to `internal/infra` and this design, rather than leaving them as pure aspiration — keeps the design spec from silently drifting from reality (existing convention already treats this file as living design documentation, not a historical artifact). |

### meshery/schemas (schema-first, land BEFORE MeshSync/Operator changes)

| File | Change |
|---|---|
| `schemas/constructs/v1beta1/connection/connection_helper_ext.go` — actually **hand-written helper, sibling to `connection_helper.go`**, e.g. `discovery_config.go` | New typed sub-contract mirroring `MeshsyncDeploymentMode`'s pattern exactly: `DiscoveryConfigMetadataKey = "meshsync_discovery_config"`, `type DiscoveryDepth string` (`"shallow"`,`"full"`), `type DiscoveryTierConfig struct { Depth DiscoveryDepth; EnabledInfra []string; DisabledInfra []string }` (JSON camelCase per the casing contract), `DiscoveryConfigFromMetadata(metadata core.Map) (DiscoveryTierConfig, bool)`, `SetDiscoveryConfigToMetadata(metadata core.Map, cfg DiscoveryTierConfig)`. This is metadata-nested (not a new top-level `Connection` field) for the same reason `MeshsyncDeploymentMode` is: it avoids a `connection.yaml` schema version bump and an `additionalProperties: false` entity-schema change for what is, for now, an operational knob rather than a first-class identity attribute of a Connection. |
| `models/v1beta1/connection/discovery_config_test.go` | Unit tests mirroring `meshsync_deployment_mode_test.go`'s structure exactly (`FromMetadata`, `SetXToMetadata`, round-trip, unknown-key, wrong-type cases). |
| `schemas/constructs/v1alpha1/crd/meshsync/` (new construct, if the team wants the CRD's `watch-list`/tier config formally schema'd rather than left as an untyped ConfigMap — **recommended**, not optional, because the raw CRD OpenAPI validation today is fully untyped per §2) | `tier_config.yaml`: response/CRD-shape schema for `{tiers: [{tier: int, depth: string, resources: [{resource: string, events: [string]}]}], infra: [{name: string, enabled: bool}]}`. This becomes the typed replacement for today's opaque `watch-list: corev1.ConfigMap`, generated into a Go struct meshery-operator's CRD types embed directly instead of `corev1.ConfigMap`. |
| `docs/casing-rules.md` / `docs/schema-review-checklist.md` | No change needed if the new construct follows existing camelCase-on-newly-authored-version convention; verify via `make validate-schemas` per repo convention. |
| Run `make build && make validate-schemas && make consumer-audit` | Required before any downstream repo consumes the new types, per `meshsync/CLAUDE.md`'s "Schema-aware changes" rule and `schemas/AGENTS.md`'s "Required on Every PR." |

**Why metadata-nested for the Connection-level knob but a real typed schema for the CRD.** The Connection-level setting is a per-connection *override* of depth/scope (user says "for cluster X, run shallow only") — it rides on the existing free-form `Metadata` exactly like deployment mode does, no entity schema change, fast to ship. The CRD-level `watch-list` is the actual operational contract MeshSync reads to build its pipelines — it deserves a typed schema now that it's growing a third dimension (tier) beyond the original two (whitelist/blacklist), because an untyped `corev1.ConfigMap` with implicit key names (`"whitelist"`, `"blacklist"`, now `"tier-config"`) is exactly the kind of latent fragility CLAUDE.md's "pay down technical debt as you encounter it" calls out — but this is significant enough surface area that I'm flagging it as a **maintainer decision** (see §11) rather than silently expanding scope, since it touches the CRD's raw OpenAPI validation schema (`crds.yaml`) which is generated/vendored across `meshery-operator`, `meshery/install/kubernetes/helm`, and `meshery/models/meshery-operator/*` — a wide blast radius for a "nice to have" typing improvement.

### Meshery Operator

| File | Change |
|---|---|
| `api/v1alpha2/meshsync_types.go` | **v1alpha2 only** (v1alpha1 stays frozen — it's the non-storage version; per existing convention `+kubebuilder:storageversion` marks v1alpha2 as authoritative). Add `TierConfig *runtime.RawExtension` (or, if the schemas construct above lands, the generated typed struct) to `MeshSyncSpec`, sibling to `WatchList`. Keep `WatchList` for back-compat; new field is additive and optional. |
| `api/v1alpha2/zz_generated.deepcopy.go` | Regenerate via `make manifests`/controller-gen (existing project convention — do not hand-edit `zz_generated.*`). |
| `api/v1alpha1/conversion_test.go` and a new `api/v1alpha1/*_conversion.go` (if a `Hub`/`Spoke` conversion webhook exists between v1alpha1↔v1alpha2 — verify against `conversion_test.go`'s existing coverage) | Extend conversion to round-trip the new field (defaulting to nil/empty when converting from v1alpha1, which has no such field). |
| `config/crd/bases/meshery.io_meshsyncs.yaml`, `bundle/manifests/meshery.io_meshsyncs.yaml`, and the **duplicated, vendored copies** in `meshery/install/kubernetes/helm/meshery-operator/{crds,files}/crds.yaml` | Regenerate CRD YAML (controller-gen) to include the new field's OpenAPI validation schema. **Note the cross-repo duplication already visible in §2** — `meshery/meshery`'s installer vendors a copy of the CRD YAML rather than referencing meshery-operator's; this PR must update both, and this is a pre-existing coupling worth flagging to maintainers as tech debt (a single point of drift risk that predates this feature). |
| `pkg/meshsync/meshsync.go` | No change required for the MVP — the Deployment shape doesn't change; MeshSync reads `Spec.TierConfig` directly from the CR itself (same pattern as `WatchList` today), not via an env var or volume mount. |
| `docs/architecture.md` | Document the new field per that repo's own "documentation is part of the change" convention (mirrors MeshSync's CLAUDE.md rule). |

### Meshery Server

| File | Change |
|---|---|
| `handlers/connections_handlers.go` | In `UpdateConnectionById` (lines 359-468), add a parallel check to the existing `MeshsyncDeploymentModeFromMetadata` block (lines 386-426): `if cfg, ok := connections.DiscoveryConfigFromMetadata(connection.MetaData); ok { h.handleDiscoveryConfigChange(...) }`. New `handleDiscoveryConfigChange` (new file `handlers/meshsync_discovery_config.go`) mirrors `handleMeshSyncDeploymentModeChange`'s shape: fetch the K8s connection's underlying MeshSync CR (via the existing K8s client already used for CR reads elsewhere), patch `Spec.TierConfig`, and — critically, since MeshSync's new `WatchMeshSyncCR` (see MeshSync changes) picks up CR changes on its own — Server does **not** need to also publish a `ReSyncDiscoveryEntity` broker message for this path; the CR watch is sufficient and avoids a double-trigger race. (Contrast with the existing `Resync()`/`ResyncMeshsync` path, which remains for the CRD-definition-change case and any manual "resync now" UI action.) |
| `models/meshery_controllers.go` | No structural change; `MesheryControllersHelper.ResyncMeshsync` stays as the manual-resync path, unaffected. |
| `handlers/connection_definition_handler.go` or a new `handlers/meshsync_discovery_handler.go` | New read endpoint: `GET /api/integrations/connections/{connectionId}/meshsync/discovery-config` returning the current effective `DiscoveryTierConfig` for that connection (read from the CR live, not cached) — needed so the UI can show current state before offering a change. |
| `internal/graphql/model/operator_helper.go` | If MeshSync status/discovery state should be exposed via the existing GraphQL operator subscription (the doc's §"Graphql Subscriptions" already covers `operatorStatus`/`meshsyncStatus`), extend the resolver to surface `tierConfig`/detected-infra summary — optional for MVP, listed as a phase-2 item (§10). |

### Meshery UI

| File | Change |
|---|---|
| `rtk-query/connection.ts` | New mutation `updateMeshsyncDiscoveryConfig` (POST/PUT to the new Server endpoint) and query `getMeshsyncDiscoveryConfig`, following the exact pattern of `updateConnectionById`/`getConnectionDetails` (lines 45-50, 72-82) — invalidate `Connection_API_Connections` or a new dedicated tag if the read is not embedded in the connection list payload. |
| `components/connections/ConnectionTable.hooks.ts` | New `useDiscoveryConfigActions` hook (sibling to `useConnectionActions`) wrapping the new mutation with notifications, following the established `try/catch` + `notify()` shape (lines 23-45). |
| `components/connections/` (new component, e.g. `DiscoveryConfigDialog.tsx`) | A settings dialog per-connection (opened from the existing connection row actions menu) offering: depth toggle (shallow/full), a checklist of detected/known infra with enable/disable, and a "resync now" button (wired to the existing resync mutation, not a new one). |
| `components/connections/ConnectionTable.types.ts` | Add the `DiscoveryTierConfig` TS type (generated from schemas' TypeScript output, per the existing `@meshery/schemas` RTK-generated-client pattern already used for `Connection` itself in `rtk-query/connection.ts:1-4`). |

### MeshKit

| File | Change |
|---|---|
| `broker/messaging.go` | Add `InfraDetected ObjectType = "infra-detected"` and, if Tier-3 delegation needs a distinct request/reply shape beyond a plain publish, a `RequestEntity` constant `InfraDiscoveryRequestEntity = "infra-discovery"`. This is additive to the existing const block (lines 3-23) — no existing constant changes. |
| No changes to `broker/broker.go`'s `Handler` interface | `PublishInterface`/`SubscribeInterface` (already generic) are sufficient for adapters to both publish detection acks and subscribe to detection events; no new methods needed. |

## 5. Schema/CRD/API/Model Changes — Summary Table

| Layer | Field | Type | Version | Back-compat |
|---|---|---|---|---|
| `meshery/schemas` — `Connection.Metadata` sub-contract | `meshsync_discovery_config` key → `DiscoveryTierConfig{depth, enabledInfra, disabledInfra}` | New hand-written helper (`connection_helper.go` sibling), camelCase JSON | v1beta1 `connection` construct, additive (no entity-schema field added, no version bump) | Absent key ⇒ `ok=false` from `FromMetadata`, caller treats as "no override," full-depth default preserved |
| `meshery/schemas` (recommended, flagged as open question) — new CRD-tier construct | `TierConfig{tiers[], infra[]}` | New typed schema, `constructs/v1alpha1/crd/meshsync/` | New, v1alpha1 | Purely additive sibling to `WatchList`; old CRs with only `WatchList` populated continue to work unmodified |
| `meshery-operator` CRD | `MeshSyncSpec.TierConfig *TierConfig` (or `RawExtension` pending schemas decision) | Go struct field | `v1alpha2` only (storage version); `v1alpha1` untouched | Optional field, nil-safe; v1alpha1→v1alpha2 conversion defaults to nil |
| MeshSync `internal/config` | `PipelineConfig.Depth`, `PipelineConfig.Tier`; `MeshsyncConfig.Discovery *DiscoveryConfig` | Go struct fields, JSON/YAML tags following this repo's existing mixed style (new fields MUST be camelCase per `docs/agent-instructions/naming-conventions.md`) | N/A (internal config, not wire) | Zero-value `Depth`/`Tier` treated as legacy full-fidelity; whitelist/blacklist path fully preserved as the default when no tier config present |
| MeshSync wire model (`pkg/model.KubernetesResource`) | **No field changes.** Depth trims *values*, not the struct shape. | Unchanged | N/A | Full back-compat: `meshery.meshsync.core` continues to carry the same struct; Server's GORM layer needs zero changes for MVP |

**Versioning stance:** no MeshSync wire-protocol version bump, no new NATS subject for the core stream. The only new subject is `meshery.meshsync.infra-detected` (net-new, adapters opt in), so it cannot break any existing subscriber. CRD gets a field addition on the already-existing `v1alpha2` storage version, not a new CRD version — Kubernetes' own additive-field compatibility rules apply (old clients/controllers ignore unknown fields safely).

## 6. Cross-Repo Sequencing & Feature-Flagging

1. **`meshery/schemas` first.** Land `discovery_config.go` helper + tests. Run `make build && make validate-schemas && make consumer-audit`. Tag/release per the repo's `meshery-schemas-release` skill procedure (never hand-cut). This unblocks everyone else importing the new symbols.
2. **`meshery-operator` second**, consuming the new schemas release (or, if the CRD-tier construct is deferred per the open question in §11, operator adds `TierConfig *runtime.RawExtension` directly with no schemas dependency for the MVP). Regenerate CRDs, update both the in-repo `config/crd/bases` and coordinate a follow-up PR in `meshery/meshery`'s vendored `install/kubernetes/helm/meshery-operator/crds/crds.yaml` copy — **do not let these drift**, per the "documentation is part of the change" and "multi-repo awareness" rules.
3. **MeshSync third**, gated behind a feature flag: a new CLI flag `-tieredDiscovery` (default `false` for the MVP release, matching the conservative rollout the "back-compat" rule demands) or, more in line with existing conventions, gate purely on **presence of the new CR field** — if `Spec.TierConfig` is nil/absent (which it will be for every existing deployed CR until an operator/user opts in), MeshSync falls through to today's whitelist/blacklist path unchanged, with zero behavior difference. This is preferable to a CLI flag because it requires no Operator/Deployment coordination — the CR is already the per-cluster config surface. Land: schemas-typed helper consumption → `internal/config` tier types → `internal/pipeline` depth-aware trimming → `internal/infra` detection → `meshsync.WatchMeshSyncCR` hot-reload → `meshsync.tier3` conditional pipelines, each as an independently-reviewable, independently-testable PR (this repo's CLAUDE.md explicitly calls out incremental, well-tested change sets).
4. **Meshery Server fourth**, consuming both the schemas metadata helper and (if MeshSync's CR-watch path has landed) relying on it for propagation rather than re-inventing a push mechanism.
5. **Meshery UI last**, consuming Server's new endpoint.

No repo needs a simultaneous multi-repo atomic merge: every step above is designed so the **absence** of the next step's changes leaves the previous step's changes inert (new CR field ignored by an old MeshSync; new metadata key ignored by an old Server handler) — this is the safest sequencing for an ecosystem where repos deploy independently.

## 7. Back-Compat, Rollout & Migration

- **Existing whitelist/blacklist users**: zero migration required. `validateLists` keeps enforcing exactly-one-of today; tier config is a net-new, optional third path (`Discovery` field), and `PopulateConfigsFromMap` only engages tier logic when that field is present and non-nil.
- **Existing embedded/local (no-CRD) deployments** (`config_local.go`): unaffected — `GetMeshsyncCRDConfigsLocal` continues to synthesize the same flat whitelist; a future enhancement could express `LocalMeshsyncConfig`'s already-present infra CRDs (grafana, prometheus) as a proper Tier-3 gate instead of an unconditional whitelist entry, but that is not required for back-compat and is listed as phase-2 (§10).
- **Wire contract**: `meshery.meshsync.core` unchanged in shape; Server needs no schema migration for the MVP. A cluster running the new MeshSync with shallow depth for some kinds will simply publish sparser `KubernetesResourceSpec`/`Status` values for those kinds — Server persists them as-is (nullable columns already tolerate this; verify via `RemoveStaleObjects`'s `Migrator()` calls in `models/meshsync_events.go:278-305` that no `NOT NULL` constraint exists on Spec/Status subfields that shallow discovery would leave empty — **flagged for the test plan**, §9).
- **Rollout**: ship MeshSync's tier-aware code as a no-op-by-default (CR field absent ⇒ legacy path) release first; ship Operator's CRD field addition; only then does an operator/user opting in by populating `Spec.TierConfig` actually change behavior on that one cluster. This is a naturally per-cluster progressive rollout with no global flag needed.
- **Migration path for the "recommended, flagged" typed CRD schema** (§11): if adopted, migrate `WatchList corev1.ConfigMap` → typed struct as a v1alpha2-only, additive field; existing `WatchList` stays valid indefinitely (dual-read: MeshSync tries `TierConfig` first, falls back to parsing `WatchList` as today) rather than a breaking cutover.

## 8. Risks, Failure Modes, Perf/Scale

- **Tier-3 in-process pipelines double informer/watch load on the API server per detected infra.** Mitigate: Tier-3 pipelines only start for infra actually detected (gated), and each Tier-3 pipeline's own resource set should be small and infra-scoped (e.g., Istio's `VirtualService`/`Gateway`/`DestinationRule`, not "everything"). Document expected additional watch count per infra signature so cluster operators can budget for it.
- **Race between `WatchMeshSyncCR`'s hot-reload and an in-flight `ReSync` from the CRD-definition-watch path.** Both ultimately write `config.ResourcesKey`/new `config.DiscoveryKey` and signal the same `channels.ReSync` channel. The existing `debounce` in `meshsync/discovery.go` (5s, wrapping `debouncedRestartDiscovery`) already coalesces bursts from any signal source — verify (test plan item) that a CR change arriving mid-debounce doesn't get silently dropped rather than coalesced, since the debounce closure currently only reads `currentPipelineCh`'s config once triggered, not per-signal.
- **Tier-3 stage lifecycle leak.** Each detected infra spins its own stop channel; if `h.tier3Managers` isn't drained on `Handler.Run`'s shutdown path (mirroring the existing `defer meshsyncHandler.ShutdownInformer()` in `pkg/lib/meshsync/meshsync.go:219`), a shutdown leaves Tier-3 informer goroutines running. Requires an explicit `ShutdownTier3Pipelines()` called from the same defer chain.
- **Adapter-delegated Tier 3 has no liveness contract today.** The design spec itself flags this as an open question ("How will the adapters know when to connect to NATS? ... Initially we are going with infinite tries" — line 384-385). If an adapter is not running, a `meshery.meshsync.infra-detected` publish is a fire-and-forget with no reply; MeshSync should not block or retry indefinitely waiting for adapter uptake — publish once per detection-state-transition (not on every resync) and let the adapter's own reconnect/backoff handle availability, matching MeshSync's own `WatchCRDs` backoff pattern (`meshsync/handlers.go:326-364`) as the idiomatic precedent already in this codebase.
- **Depth trimming silently changing Server-side derived data.** Server's `getComponentMetadata` (`models/meshsync_events.go:322-364`) and downstream Kanvas topology rendering may implicitly assume `Spec`/`Status` are always populated for certain kinds. Shallow depth should default to **Tier 2 (full)** for any kind Server/UI is known to render richly today (Pods, Deployments, Services) and only default new/rarely-rendered kinds to shallow — i.e., the *default* tier assignment in `default_config.go` should be conservative, not blanket-shallow, to avoid a silent UX regression. This is a design decision to make explicitly per-kind, not a blanket toggle.
- **Scale claim validation.** The stated primary lever for large clusters is Tier 1 shallow scanning avoiding full-object informer cache memory (client-go informer caches hold full objects regardless of what's published — trimming at `publishItem` does not reduce informer memory, only NATS payload size and DB row width). **This is an important nuance to flag to maintainers**: true API-server/memory scale relief requires field-selector-narrowed or metadata-only informers (a separate, larger change to `dynamicinformer.DynamicSharedInformerFactory` usage, e.g., via `metav1.PartialObjectMetadata` list/watch) — out of scope for this design but the natural "full" answer to the "scale" requirement in the prompt. This blueprint's depth trimming reduces publish/DB cost, not watch/cache cost; call this out explicitly rather than overclaiming.
- **Interaction with the separately-planned namespace-scoped informers enhancement.** That work (per the prompt, "a separate planned enhancement") would change `GetDynamicInformer`'s factory construction to be per-namespace rather than cluster-wide. Tier gating is orthogonal (it selects *which kinds*, namespace-scoping selects *which namespaces*) but both mutate `internal/pipeline.New`'s call signature and `meshsync.Handler`'s informer lifecycle — sequencing note for whichever lands second: rebase pipeline signature changes against the other, do not let both add positional parameters independently (use an options struct if both are in flight concurrently).

## 9. Test Plan Per Repo (+ runtime verification)

**MeshSync**
- `internal/config`: unit tests for `populateTierConfig` (new key present/absent/malformed JSON), `PopulateConfigsFromMap` back-compat (existing whitelist/blacklist tests in `crd_config_test.go` must still pass unmodified), and depth-defaulting logic.
- `internal/infra`: unit tests for `Detect` against synthetic `cache.Store` fixtures (infra present, infra absent, partial CRD match).
- `internal/pipeline`: extend `handlers_test.go`/`pipeline_test.go` for depth trimming (`publishItem` output has empty Spec/Status for shallow, full for full) and for `pipeline.New`'s new parameter threading.
- `meshsync/handlers_test.go`: new test for `WatchMeshSyncCR`'s change-detection (mirrors the existing `resyncSignaled`/CRD-event test pattern at line 232-250) — verify a `MODIFIED` event on the MeshSync CR with an actual `TierConfig` diff signals `ReSync`, and a no-diff `MODIFIED` event does not (same discipline as `updatePipelineConfig`'s existing `changed` bool).
- `meshsync/tier3_test.go` (new): verify Tier-3 pipeline start/stop lifecycle on detect/undetect, and shutdown cleanup.
- Run `make test` (lint + `go test -race`) — all must pass locally before requesting review, per CLAUDE.md.
- **Integration tests** (`integration-tests/`): new scenario — a kind cluster with a synthetic CRD matching one seeded `InfraSignature`, verifying the Tier-3 in-process path activates and publishes Tier-3-scoped resources; a second scenario with `DelegateToAdapter: true` verifying the `infra-detected` publish fires and no Tier-3 pipeline starts in-process.
- **Runtime verification**: use the `verifier-meshsync` skill against a local kind cluster with Prometheus Operator's CRDs installed (or another seeded signature) to confirm live detection + Tier-3 activation end-to-end, and to confirm `WatchMeshSyncCR` picks up a live CR edit without a pod restart (the concrete proof of closing the gap identified in §1).

**meshery/schemas**
- `discovery_config_test.go` mirroring `meshsync_deployment_mode_test.go` exactly (from-metadata, set-to-metadata, unknown key, wrong type).
- `make build && make validate-schemas && make consumer-audit` — mandatory before release, per repo convention.

**meshery-operator**
- Extend `api/v1alpha1/conversion_test.go` (or add `v1alpha2` equivalent) for round-tripping the new field.
- `controllers/meshsync_controller_test.go`: verify reconciliation is a no-op change to the Deployment when only `TierConfig` changes (it should not roll the Deployment — MeshSync reads the CR directly, not via env/volume), confirming the reconciler doesn't over-react.

**Meshery Server**
- Unit tests for `handleDiscoveryConfigChange` mirroring the existing `handleMeshSyncDeploymentModeChange` test coverage shape.
- Handler test for the new discovery-config read endpoint.

**Meshery UI**
- RTK Query mutation tests mirroring `rtk-query/__tests__/connection.test.ts`'s existing pattern.
- Component test for the new dialog (render, submit, error path) following existing `ConnectionWizard.helpers.test.ts` conventions.
- e2e: extend `tests/e2e/connections.spec.ts` with a discovery-config-change scenario if the existing suite covers connection-settings mutation flows.

## 10. Effort Estimate & Phasing (MVP → Full)

**MVP (ship first, smallest coherent slice):**
1. schemas: `DiscoveryTierConfig` metadata helper (~0.5 day incl. tests/release).
2. MeshSync: `Depth`/`Tier` fields on `PipelineConfig`, depth trimming in `publishItem`, tier config parsing with full whitelist/blacklist back-compat (~2 days incl. tests).
3. MeshSync: `WatchMeshSyncCR` hot-reload path (~1.5 days incl. tests) — **this alone is independently valuable** (closes a real operability gap even without tiers) and should be considered for extraction as its own earlier PR.
4. meshery-operator: additive `TierConfig` field on v1alpha2 + CRD regen across both repos' vendored copies (~1 day).
5. Server: metadata-driven discovery-config-change handler + read endpoint, reusing the CR-watch for propagation (~1.5 days).
6. UI: minimal depth toggle (shallow/full) in the connection settings dialog, no infra checklist yet (~1 day).

Estimated MVP: **~7.5-8 engineer-days** across repos, sequenced per §6, likely 2-3 calendar weeks accounting for cross-repo review/release cadence (schemas release process alone has its own cadence per `meshery-schemas-release`).

**Full (subsequent phases):**
- Phase 2: `internal/infra` detection + conditional in-process Tier-3 pipelines for 2-3 seeded signatures (istio, prometheus-operator) (~3-4 days).
- Phase 3: adapter-delegated Tier-3 (`infra-detected` NATS subject in MeshKit, adapter-side subscription contract, documentation for adapter authors) (~3 days, plus adapter-repo-side work not estimated here since no adapter repo was in scope).
- Phase 4: UI infra-checklist + detected-infra display (surfacing what Tier 1 found even before a user opts into Tier 3) (~2 days).
- Phase 5 (stretch, explicitly flagged as a *different* mechanism than this design delivers): true watch-level scale relief via partial-object-metadata informers for Tier 1, which is the only way to actually reduce API-server/cache memory pressure rather than just NATS/DB payload size (see §8's scale-nuance risk) — this deserves its own design pass, not a bolt-on to this one.

## 11. Open Questions Needing Maintainer Decisions

1. **Typed CRD schema vs. `RawExtension`.** Should the CRD's tier config be a fully-typed `meshery/schemas` construct (recommended for consistency and CI-enforced validation) or ship faster as `runtime.RawExtension` in `meshery-operator` alone, deferring the schemas migration? This changes the sequencing in §6 materially (typed-first adds a schemas release to the critical path).
2. **Default depth per resource kind.** Should the initial `default_config.go` tier/depth assignment be "everything full by default, opt into shallow" (safest, zero behavior change) or "cheap/high-volume kinds (Pods, Events, EndpointSlices) shallow by default, opt into full" (more scale benefit out of the box, but a behavior change for existing clusters the moment they upgrade and populate any `TierConfig`)? I've assumed the former (opt-in shallow) in this blueprint; confirm.
3. **Who owns the initial infra-signature registry's contents and update cadence?** `internal/infra/signatures.go` needs the same "how do we keep MeshSync not fragile against ecosystem drift" concern the design spec itself raises (line 310: "How do we keep the Operator up to date with new [infrastructure]-specific custom resources?"). Should signatures be sourced from MeshKit's `registry` package (which already models component/relationship registries) rather than hardcoded in MeshSync, so adapters can self-register their own detection signature instead of MeshSync needing a code change per new supported infra?
4. **Does Tier-3 delegation to an adapter require MeshSync to also know which adapters exist/are healthy**, or is a bare fire-and-forget publish (per §8) sufficient for the MVP? The design spec explicitly defers this ("Initially we are going with infinite tries") — confirm that's still the accepted answer today, three+ years after that spec was written, given Meshery's adapter model may have evolved.
5. **Should `WatchMeshSyncCR`'s hot-reload be shipped as its own standalone PR/release ahead of the tiering work**, given it fixes a real, independently-valuable operability gap (CR edits currently require a pod restart)? I've bundled it into the MVP above, but it could ship first and separately.

---

**Key files referenced (absolute paths):**

- `internal/config/default_config.go`
- `internal/config/crd_config.go`
- `internal/config/types.go`
- `internal/config/config_local.go`
- `internal/pipeline/pipeline.go`
- `internal/pipeline/step.go`
- `internal/pipeline/handlers.go`
- `meshsync/handlers.go`
- `meshsync/discovery.go`
- `pkg/lib/meshsync/meshsync.go`
- `docs/design-spec_meshsync-infrastructure-synchronization.md`
- `docs/agent-instructions/architecture.md`
- `docs/agent-instructions/errors.md`
- `docs/agent-instructions/naming-conventions.md`
- `helpers/component_info.json`
- `meshery-operator/api/v1alpha1/meshsync_types.go`
- `meshery-operator/api/v1alpha2/meshsync_types.go`
- `meshery-operator/controllers/meshsync_controller.go`
- `meshery-operator/pkg/meshsync/meshsync.go`
- `schemas/models/v1beta1/connection/connection.go`
- `schemas/models/v1beta1/connection/connection_helper.go`
- `schemas/models/v1beta1/connection/meshsync_deployment_mode_test.go`
- `schemas/schemas/constructs/v1beta1/connection/connection.yaml`
- `schemas/AGENTS.md`
- `meshery/server/models/meshsync_events.go`
- `meshery/server/models/meshery_controllers.go`
- `meshery/server/models/connections/connections.go`
- `meshery/server/handlers/connections_handlers.go`
- `meshery/server/machines/kubernetes/resync_resources.go`
- `meshery/server/internal/graphql/model/operator_helper.go`
- `meshery/ui/rtk-query/connection.ts`
- `meshery/ui/components/connections/ConnectionTable.hooks.ts`
- `meshery/install/kubernetes/helm/meshery-operator/crds/crds.yaml`
- `meshkit/broker/messaging.go`
- `meshkit/broker/broker.go`
