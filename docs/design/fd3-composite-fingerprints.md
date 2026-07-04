# Blueprint: Composite Fingerprints Spanning Multiple Entities

## 1. Goal & Gap

**Goal.** Correlate the individually-discovered Kubernetes objects that together compose one application (Deployment + Service + ConfigMap + Secret + ...) into a named group, then use signals across that group (container images, CRDs, owned Deployments) to positively identify the infrastructure/tool and its version - closing gap #4 of the "Configurable and Tiered Discovery" design (`docs/design-spec_meshsync-infrastructure-synchronization.md` lines 209-227, "Composite Prints" / Builder pattern over Images/CRDs/Deployment).

**Gap, verified in code, not just docs.**

- `pkg/model.ParseList` (`pkg/model/model_converter.go:19-104`) converts one `unstructured.Unstructured` in isolation and returns one `KubernetesResource`. There is no queue, buffer, or shared state across objects anywhere in the pipeline.
- `RegisterInformer.publishItem` (`internal/pipeline/handlers.go:82-112`) and every `output.Writer` implementation (`broker.go:19-32`, `processor.go:21-27`) take exactly one `model.KubernetesResource` per call. Even the only batch code path - the "informer-store" resync reply consumed in `MeshsyncDataHandler.subsribeToStoreUpdates` (`meshery/server/models/meshsync_events.go:134-185`) - iterates the batch and persists each object with `persistStoreUpdate` one at a time; it is a snapshot dump, not a correlation point.
- The only existing cross-field enrichment is `K8SService.Process` (`pkg/model/process.go`), and it is single-object: it derives a `connection` capability + URLs from one `Service`'s own spec/status, never looking at a sibling object.
- Server-side, `meshsyncEventsAccumulator` (`meshery/server/models/meshsync_events.go:209-252`) persists each `KubernetesResource` independently via `dbHandler.Create`/`.Updates` per event; `getComponentMetadata` (lines 320-364) classifies a resource against the MeshModel component registry one Kind+APIVersion at a time.
- `app.kubernetes.io/name|instance|part-of` labels have **zero footprint** in any processing code in MeshSync or Server - they appear only in two unrelated test fixture YAMLs in MeshSync.
- The auto-registration path already anticipates this gap and stubs around it: `AutoRegistrationHelper.processRegistration` (`meshery/server/machines/helpers/auto_register.go:57-139`) contains the comment *"Ideally iterate all Connection defs, extract fingerprint composite key and try to match with the given obj"* (line 66), and `getTypeOfConnection` (line 179-187) is a hardcoded name-substring match annotated `// Improve this fingerprinting`. This is the intended replacement point for a real fingerprint.

## 2. Current State Per Repo

### MeshSync

- `pkg/model/model.go:13-30` - `KubernetesResource` has a `Model` field (`json:"model"`) that MeshSync **never populates**; it ships empty and Server fills it in (see below). `ComponentMetadata map[string]interface{}` (`json:"component_metadata"`, snake-case, load-bearing per `docs/agent-instructions/naming-conventions.md:16`) is MeshSync's only per-object annotation channel today, populated exclusively by `K8SService.Process`.
- `pkg/model/preprocessor.go:11-18` - `GetProcessorInstance(kind string)` is a `switch` returning a `ProcessFunc` keyed on Kind; `"Service"` is the only registered case. This is the extension seam for per-Kind enrichment, but it is inherently single-object (`Process(obj []byte, k8sresource *KubernetesResource, evtype broker.EventType) error`) - no access to other resources.
- `internal/pipeline` (`step.go`, `handlers.go`) rebuilds three fresh stages (global discovery, local discovery, StartInformers) on every run/resync per `docs/agent-instructions/architecture.md:36-38` - no package-level state may be hoisted across runs.
- No label/annotation-based correlation, no per-cluster resource cache, no builder pattern over images/CRDs/Deployments exists in this repo.

### meshery/schemas

- `schemas/constructs/v1beta3/relationship/relationship.yaml` + generated `models/v1beta3/relationship/relationship.go` define `RelationshipDefinition` - `{id, schemaVersion, version, kind (hierarchical|edge|sibling), type, subType, status, capabilities[], metadata, model, modelId, evaluationQuery, selectors}`. This is a **relationship-kind template/definition**, analogous to how `ComponentDefinition` is a component-kind template - it is registered once per MeshModel model (e.g. "kubernetes") and reused everywhere that kind of relationship applies.
- The taxonomy already has the exact two constructs this feature needs, both implemented server-side against **design-time** components:
  - `kind=hierarchical, type=parent, subType=inventory` - `HierarchicalParentChildPolicy` (`meshery/server/policies/policy_hierarchical.go:10-21`), used for owner-reference-style parent/child (e.g., Deployment owns ReplicaSet owns Pod).
  - `kind=sibling, subType=matchlabels` - `MatchLabelsPolicy` (`meshery/server/policies/policy_matchlabels.go:13-152`), a bucket-by-shared-label-value grouping algorithm - the algorithmic template for app-instance grouping.
- Critically, **relationship instances only ever live inline** inside `PatternFile.Relationships []*relationship.RelationshipDefinition` (`schemas/models/v1beta1/pattern/pattern.go:129-151`) - a design document's JSON blob. There is no DB table of "relationship instance between entity A and entity B" independent of a design; `relationship_handlers.go` only serves the **registry catalog** of relationship kinds (GET-only, backed by `regv1alpha3.RelationshipFilter` / `regv1beta1.RelationshipFilter` querying `relationship_definition_dbs` joined to `model_dbs`).
- `docs/relationship-evaluation-engine-contract.md` documents the relationship-evaluation contract (`EvaluationRequest{design, options} -> EvaluationResponse{design, actions[]}`, server + WASM-in-browser engines) - this is a **design-mutation** contract (adds/removes/updates relationship rows *inside a design*), not a runtime-inventory-correlation contract. It is the wrong contract to reuse for live-cluster fingerprinting; the right thing to reuse is the `RelationshipDefinition` schema shape and taxonomy, not the evaluation engine itself.
- `connection.yaml` (`schemas/constructs/v1beta3/connection/connection.yaml:95-104`) has an open `metadata: core.Map` field and a `status` enum including `discovered` - the correct, non-breaking extensibility point for fingerprint data on a Connection.
- No "Application" MeshModel category/construct exists anywhere in `schemas/`.

### MeshKit

- `models/meshmodel/registry/v1alpha3/relationship_filter.go` and `registry/v1beta1/*_filter.go` are all read-side `entity.Entity` query filters over registry tables (components, models, categories, connections, relationships-as-definitions). None of them models "relationship instance between two runtime rows."
- No generic label-selector-matching helper exists outside Kubernetes-specific packages (`models/controllers/meshsync.go`, `utils/kubernetes/expose/`); server's `policies/policy_matchlabels.go` is design-time and operates on `component.Component.Configuration` JSON paths, not raw K8s object labels.

### Meshery Server

- `MeshsyncDataHandler.meshsyncEventsAccumulator` (`meshery/server/models/meshsync_events.go:209-252`) is the sole per-event ingestion point: classify (`getComponentMetadata`) then persist, one `KubernetesResource` at a time, for every `Add`/`Update`/`Delete`.
- `GetMeshSyncRegistrationQueue()` (`meshery/server/models/meshsync_register_queue.go`) is an in-process, unbounded-consumer channel (`chan MeshSyncRegistrationData`, buffer 10) fed by `go regQueue.Send(...)` after every `Create`/`persistStoreUpdate` call - already the async, off-critical-path hook for "do something more with this object after it's persisted."
- `AutoRegistrationHelper.processRegistration` (`meshery/server/machines/helpers/auto_register.go:57-139`) is the sole consumer of that queue today; it reads `obj.ComponentMetadata["capabilities"]["connection"]`/`["urls"]` (the exact shape `K8SService.Process` sets) and drives `machines.kubernetes`-style Connection state machines (`Register` -> `Connect` events) via `InitializeMachineWithContext`.
- `policies/policy_hierarchical.go` and `policy_matchlabels.go` are design-evaluation-only; they never touch the `meshsyncmodel.KubernetesResource` inventory tables.

### Meshery UI

- `components/dashboard/resources/config.tsx` organizes the "Cluster Resources" browser purely by Kubernetes Kind (Workload/Configuration/Network/Security/Storage/CRDS) - flat per-kind tables, zero cross-object application grouping.
- No topology/graph component consumes MeshSync-discovered resources today; the only Cytoscape-style graph consumers are the design canvas (works over `PatternFile.Components`/`Relationships`, i.e., authored designs) - confirmed by the near-total absence of `meshsync`/`app.kubernetes.io` hits under `ui/components`.

## 3. Proposed Architecture

### Where correlation runs: Server, not MeshSync

**Recommendation: Server-side aggregation**, triggered incrementally by the existing registration-queue hook, not a new MeshSync pipeline stage.

Trade-off analysis:

| | MeshSync-side correlation | Server-side correlation (chosen) |
|---|---|---|
| Data completeness | Partial by construction: informer events arrive out of order relative to sibling objects (a ConfigMap can be created before its Deployment, or vice versa); MeshSync would need to buffer-and-wait per app-instance key with an unbounded/heuristic timeout, re-implementing a windowing problem MeshSync has no infrastructure for (`internal/pipeline` explicitly forbids cross-run state - `docs/agent-instructions/architecture.md:36`) | Server already has the full row set: `SELECT ... WHERE cluster_id=? AND labels @> 'app.kubernetes.io/instance=X'` sees every object that has arrived so far, and Update/Delete events re-trigger re-grouping incrementally |
| Wire contract stability | Requires either a new batched NATS subject or a new field on every `KubernetesResource` risking a breaking change to `meshery.meshsync.core` (explicitly disallowed without coordination per the ecosystem rules) | Zero wire-contract change; reuses the existing `meshery.meshsync.core` per-object stream and the existing async registration-queue side-channel |
| Per-cluster locality | Correlation happens where the labels are (arguably "closer" to the source) | Server already receives one broker connection **per managed cluster** (MeshSync is one-instance-per-cluster per `docs/agent-instructions/architecture.md:60-64`); grouping keys are naturally scoped by the existing `cluster_id` column, so locality is not actually lost - it is just deferred one hop |
| Failure isolation | A correlation bug crashes/stalls the discovery agent that also does the primary job (watching + publishing); MeshSync must stay minimal and resilient per its design goals ("Speed - the implementation should be event-driven") | A correlation bug in Server degrades a value-add feature (grouping/fingerprinting) without affecting discovery or persistence, which already succeeded before correlation runs |
| Reuse of existing constructs | None - MeshSync would need a net-new grouping/relationship model | Direct reuse: `RelationshipDefinition` registry, `regQueue`/`AutoRegistrationHelper` async pattern, `MatchLabelsPolicy`'s bucket-by-label-value algorithm (port the algorithm, not the design-time code), `HierarchicalParentChildPolicy`'s taxonomy, Connection's `discovered` status and open `metadata` field |
| Multi-tenancy / scale-out | N/A (per-cluster, single-tenant) | Server is the system of record across all connected clusters; only Server can decide "is this the same logical application deployed across two clusters" if ever needed - out of scope for MVP but architecturally available only server-side |

MeshSync's only change is a **narrow, additive** one: guarantee that `app.kubernetes.io/name`, `/instance`, `/part-of` (and `/version`, `/managed-by`) - which are already captured generically as `KubernetesResourceMeta.Labels` via `jsonparser.ObjectEach` in `ParseList` (`model_converter.go:31-44`) - are not dropped or truncated, and (Phase 2) add a builder-pattern `Processor` that extracts image/CRD signals onto `ComponentMetadata` for Kinds where that's cheap and local (Deployment/Pod/CRD), mirroring `K8SService.Process`'s existing single-object enrichment pattern exactly. MeshSync does **not** buffer, correlate, or hold cross-object state.

### Data flow

```
MeshSync (per cluster, unchanged shape)
  ParseList(obj) -> KubernetesResource{ Labels: [...app.kubernetes.io/instance=X...], ComponentMetadata: {...} }
  (Phase 2: GetProcessorInstance also matches "Deployment"/"Pod"/CRD Kinds -> BuilderSignals in ComponentMetadata)
        |  publish, unchanged, on meshery.meshsync.core (one object per message)
        v
Meshery Server: MeshsyncDataHandler.meshsyncEventsAccumulator (unchanged: classify + persist one row)
        |  go regQueue.Send(MeshSyncRegistrationData{Obj: obj})   <- existing async hook, unchanged signature
        v
NEW: AppGroupingConsumer (new goroutine, same fan-out pattern as AutoRegistrationHelper, reads the same RegChan
     or a second subscriber added to MeshSyncRegistrationQueue - see Cross-repo sequencing for the exact wiring)
        |
        |  1. Extract obj's app.kubernetes.io/name|instance|part-of (+ ownerReferences) from KubernetesResourceMeta
        |  2. Compute grouping key = (cluster_id, namespace, app.kubernetes.io/instance) with /part-of and /name as fallback tiers
        |  3. UPSERT an ApplicationGroup row (composite key) if it doesn't already exist for this key
        |  4. UPSERT a RelationshipInstance row: {relationshipDefinitionId: <sibling.matchlabels or hierarchical.inventory.parent>,
        |                                        fromEntityId: obj.ID, toEntityId: <group's anchor Deployment, if resolved>,
        |                                        groupId: <ApplicationGroup.ID>}
        |  5. (Phase 2) Run BuilderIdentifier over every KubernetesResource currently in the group
        |     (images seen on Pod/Deployment specs, CRD Kinds present, well-known Deployment names)
        |     -> resolves {tool, version} -> written to ApplicationGroup.identifiedAs
        v
   ApplicationGroup + RelationshipInstance rows (new tables, schema-first in meshery/schemas)
        |
        +--> Connection state machine: AutoRegistrationHelper's getTypeOfConnection(...) replaced by
        |    a lookup against ApplicationGroup.identifiedAs (real fingerprint) instead of substring match;
        |    Connection.metadata carries {applicationGroupId, groupMembers[], identifiedAs}
        |
        +--> GraphQL/REST: new endpoint(s) surfacing ApplicationGroup + its members + relationships
        |
        v
   Meshery UI: new "Applications" grouping view (peer to existing Kind-based Cluster Resources view) +
               topology graph fed by RelationshipInstance edges (compound/parent nodes via ApplicationGroup)
```

## 4. Per-Repo Changes

### meshery/schemas (schema-first, land before any consumer)

- `schemas/constructs/v1beta1/application_group/application_group.yaml` (**new construct**) - the grouping anchor. Fields: `id`, `schemaVersion`, `clusterId`, `namespace`, `groupingKey` (the label value used, e.g. the `app.kubernetes.io/instance` value), `groupingLabel` (which label tier matched: `instance`|`part-of`|`name`|`owner-chain`), `memberIds []Uuid` (the `KubernetesResource.ID`s in the group - MeshSync's model isn't in schemas yet, so this is a plain UUID/string reference, not a typed FK to `component.ComponentDefinition`), `identifiedAs` (nullable object: `{tool, version, confidence, evidence[]}` - the Phase-2 builder output), `createdAt`, `updatedAt`.
- `schemas/constructs/v1beta1/relationship_instance/relationship_instance.yaml` (**new construct**) - the runtime edge, deliberately thin and reusing the existing `RelationshipDefinition` catalog rather than duplicating kind/type/subType: `id`, `schemaVersion`, `relationshipDefinitionId` (FK to the existing `relationship_definition_dbs.id` - reuse `hierarchical.parent.inventory` and `sibling.*.matchlabels`, do not mint new kinds), `fromEntityId`, `toEntityId` (both plain string/UUID references to `KubernetesResource.ID` - deliberately untyped since MeshSync's model isn't schema-sourced), `applicationGroupId` (nullable FK to `application_group`), `clusterId`, `status` (`identified`|`approved`|`stale`|`deleted` - reuse the existing `RelationshipDefinitionStatus` enum values where they overlap), `createdAt`, `updatedAt`.
- Generate Go models the standard way this repo already does (`models/v1beta1/application_group/`, `models/v1beta1/relationship_instance/`) - do not hand-write structs; follow the existing `oapi-codegen` pipeline used for every other construct (mirrored by `relationship.go`'s generated-file header).
- `schemas/schemas/constructs/v1beta1/connection/connection.yaml` - **no schema change**; document (in a code comment / PR description, not a new field) that `metadata` will carry `{applicationGroupId, identifiedAs}` for auto-registered connections. Because `metadata` is already an open `core.Map`, this is additive-by-convention, not a wire change.
- Run `make validate-schemas && make consumer-audit` before this PR merges (required by `CLAUDE.md`'s Schema-Driven Implementation policy and this repo's own consumer-audit gate).
- Versioning: both new constructs start at `v1beta1` (the repo's current "new, not yet battle-tested" convention seen in `capability`, `catalog_data`, etc.) - not `v1alpha`, since this is a deliberate, reviewed design, and not `v1beta3` since it has no prior version to inherit compatibility from.

### MeshKit

- `models/meshmodel/registry/v1beta1/application_group_filter.go` (**new**) - `ApplicationGroupFilter` implementing `entity.Entity`'s `Get`/`GetById`, mirroring `connection_filter.go`'s structure exactly (filter by `clusterId`, `namespace`, `groupingKey`, pagination fields).
- `models/meshmodel/registry/v1beta1/relationship_instance_filter.go` (**new**) - same pattern, filter by `clusterId`, `applicationGroupId`, `relationshipDefinitionId`, `fromEntityId`/`toEntityId`.
- No changes to `errors/`, `broker/`, or `utils/` are required for the MVP; if Phase 2's builder needs a shared image-reference parser (e.g., extracting `repo:tag` -> tool/version heuristics), add it under `meshkit/utils/` as a new pure function (`utils.ParseImageReference` or similar) so both Server (grouping) and any future MeshSync-side Phase-2 enrichment can share it without duplicating logic - this is the one piece of "shared logic belongs in MeshKit" the ecosystem rule calls for.

### Meshery Server

- `models/application_group.go` (**new**) - CRUD helpers over the new `application_group` table (uses `meshkit/database` the same way `models/meshsync_events.go` does), plus `FindOrCreateApplicationGroup(clusterID, namespace, groupingKey, groupingLabel string) (*applicationgroupv1beta1.ApplicationGroup, error)` and `AddMember(groupID, resourceID string) error`.
- `models/relationship_instance.go` (**new**) - CRUD helpers over `relationship_instance`, plus `UpsertRelationshipInstance(defID, fromID, toID, groupID, clusterID string) error` (idempotent on `(relationshipDefinitionId, fromEntityId, toEntityId)`).
- `models/app_grouping.go` (**new**) - the correlation logic itself:
  - `ExtractGroupingLabels(obj meshsyncmodel.KubernetesResource) (key, tier string, ok bool)` - reads `obj.KubernetesResourceMeta.Labels` (already a `[]*KubernetesKeyValue`), checks `app.kubernetes.io/instance` first, falls back to `app.kubernetes.io/part-of`, then `app.kubernetes.io/name`; returns `ok=false` if none present (label-absence is explicitly a "no correlation" outcome, never a guess).
  - `ProcessForGrouping(mh *MeshsyncDataHandler, obj meshsyncmodel.KubernetesResource) error` - the consumer function, called from the registration queue (see sequencing below): resolves/creates the `ApplicationGroup`, upserts a `sibling.matchlabels` `RelationshipInstance` from this object to the group's other current members (mirroring `MatchLabelsPolicy`'s bucket-by-shared-value idea, but against DB rows instead of an in-memory `PatternFile.Components` slice - i.e., `SELECT id FROM kubernetes_resources kr JOIN kubernetes_key_values ... WHERE cluster_id=? AND key='app.kubernetes.io/instance' AND value=?`), and if `obj.KubernetesResourceMeta.OwnerReferences` resolves to another already-persisted resource in the same group, additionally upserts a `hierarchical.inventory.parent` `RelationshipInstance` reusing `HierarchicalParentChildPolicy`'s taxonomy triple.
- `models/meshsync_register_queue.go` - either (a) add a second reader by making `RegChan` a broadcast (fan the same `MeshSyncRegistrationData` to both `AutoRegistrationHelper` and the new grouping consumer), or (b) simpler and lower-risk: have `meshsyncEventsAccumulator` (`models/meshsync_events.go:220-241`) call `go models.ProcessForGrouping(mh, obj)` directly alongside the existing `go regQueue.Send(...)` call, exactly as it already fires two independent goroutines' worth of post-processing per object today. **Recommend (b)** - it requires zero changes to the existing queue's consumer contract and keeps grouping and auto-registration as clearly independent side effects of persistence, matching the file's existing pattern of firing off async work inline.
- `machines/helpers/auto_register.go` - replace the `getTypeOfConnection` substring hack (lines 179-187) with a lookup against the object's resolved `ApplicationGroup.identifiedAs.tool` (falls back to the existing substring heuristic when no group/fingerprint exists yet, so behavior never regresses for ungrouped or Phase-1-only objects). Set `ConnectionPayload.MetaData["applicationGroupId"]` (`getConnectionPayload`, line 141-159) so the Connection carries a durable link back to its group.
- `handlers/application_group_handlers.go` + `handlers/relationship_instance_handlers.go` (**new**) - REST endpoints (`GET /api/system/application-groups`, `GET /api/system/application-groups/{id}`, `GET /api/system/application-groups/{id}/relationships`) following `handlers/relationship_handlers.go`'s existing shape (pagination params, `registryManager.GetEntities(...)` for definitions vs. direct `dbHandler` queries for instances).
- `internal/graphql/resolver/` - extend or add a resolver so Kanvas can subscribe to application-group changes the same way `meshsync.go`'s resolver already streams `KubernetesResource` events (`internal/graphql/resolver/meshsync.go`), so the UI graph can update live rather than polling.

### Meshery UI

- `components/dashboard/resources/applications/` (**new**) - a new top-level "Applications" entry in `ResourcesConfig` (`components/dashboard/resources/config.tsx:11-44`), analogous to `Workload`/`Configuration`/etc., whose table lists `ApplicationGroup` rows (name = `groupingKey`, member count, `identifiedAs.tool`/`version` if resolved, namespace, cluster) with drill-down to member resources.
- Topology graph: extend whatever Cytoscape-based graph component today renders `PatternFile.Components`/`Relationships` for designs to optionally accept `RelationshipInstance` edges + `ApplicationGroup` compound-node membership for the **discovered-cluster** view (a separate data source, not a repurposing of design components) - use Cytoscape's native compound-node support (parent/child grouping), which is already the chosen graph library per the design spec (`docs/design-spec_meshsync-infrastructure-synchronization.md:291-297`, "Cytoscape.js ... nodes are elements in an array and edges are elements with properties 'from' and 'to'").
- `components/layout/NotificationCenter/formatters/meshsync_events.tsx` - extend to render a distinct event/notification when a new `ApplicationGroup` is first identified (e.g., "Detected new application: my-app (3 resources)") or when `identifiedAs` resolves (e.g., "Identified my-app as Redis 7.2"), following the file's existing formatter pattern for other MeshSync event types.

## 5. Schema/Model Changes - Detail

**Schema-first sequencing is mandatory** (per `CLAUDE.md`): `meshery/schemas` PR lands and is tagged/released before any Server PR that imports the new packages compiles against it.

Reuse, explicitly:

- **Do not** invent a new "kind" enum value on `RelationshipDefinition` (`hierarchical|edge|sibling` stays exactly as-is). The two edges this feature produces map onto the existing taxonomy with zero schema change to `relationship.yaml`:
  - Application-instance peer grouping -> `kind: sibling`, `type: <existing or new subType>`, `subType: matchlabels` (already used design-time by `MatchLabelsPolicy`; the runtime feature reuses the *same* `RelationshipDefinition` row, just referenced from a new `RelationshipInstance.relationshipDefinitionId` instead of from a `PatternFile.Relationships[]` inline array).
  - Owner-reference parent/child within an app -> `kind: hierarchical`, `type: parent`, `subType: inventory` (already used design-time by `HierarchicalParentChildPolicy`).
- **New, additive-only** constructs: `application_group` and `relationship_instance` are the only new schema surfaces. Both are pure additions (new tables, new OpenAPI constructs) with no modification to any existing construct's required fields, so there is no back-compat concern for existing consumers - a server that doesn't yet know about these tables simply doesn't populate/query them.
- Back-compat / versioning: since both constructs start life at `v1beta1`, no deprecation path is needed yet. If the grouping key strategy needs to change later (e.g., adding a `bySelector` tier that matches on `spec.selector.matchLabels` instead of `metadata.labels`), that is an additive optional field on `application_group`, not a breaking change, following the same additive-first discipline the rest of `schemas/` already uses (see how `connection.yaml` has grown fields like `environments`, `styles` without ever breaking `id`/`name`/`type`/`subType`/`kind`).
- `component_info.json`/`errorutil` impact: none in `schemas` (schemas repo doesn't use MeshKit's error framework for this kind of change); MeshSync's Phase-2 builder work will allocate from `next_error_code: 1015` onward for any new `ErrBuild*`/`ErrExtractSignal*` errors in `meshsync/error.go`.

## 6. Cross-Repo Sequencing & Feature-Flagging

1. **`meshery/schemas`**: land `application_group` + `relationship_instance` constructs, generated Go models, `make validate-schemas && make consumer-audit`, tag a release.
2. **`meshkit`**: land the two new registry filters against the tagged schemas version; tag a release. (No MeshSync-facing MeshKit change needed for Phase 1.)
3. **`meshery` (server)**: bump `go.mod` to the new `schemas`+`meshkit` tags; land `models/application_group.go`, `models/relationship_instance.go`, `models/app_grouping.go`, the `meshsync_events.go` one-line hook, the `auto_register.go` fingerprint-lookup swap, and the new handlers - all gated behind a feature flag (recommend reusing the existing `viper`-backed config pattern already visible in `auto_register.go:61` (`viper.Get("INSTANCE_ID")`), e.g. `FEATURE_APPLICATION_GROUPING` env var checked once in `ProcessForGrouping`, defaulting to **off** until the UI is ready) so the DB migration and background correlation goroutine can ship dark.
4. **`meshery/ui`**: land the "Applications" dashboard view + notification formatter behind the same flag (a `/api/system/features` capability check, the ecosystem's existing pattern for gating UI-visible features), merged after Server's flag defaults are verified stable in a release or two.
5. **Flip the flag on** once Server-side grouping has run against representative clusters without perf regressions (see Section 8), then remove the flag in a follow-up cleanup PR.
6. **MeshSync Phase 2** (builder-pattern image/CRD signal extraction) is fully independent of steps 1-5's Server-side rollout and can land at any point after step 1, since it only ever *adds* keys to `ComponentMetadata` - Server already merges `ComponentMetadata` via `utils.MergeMaps` (`meshsync_events.go:222`, `234`, `258`) and any grouping logic reading it must treat new builder-signal keys as optional.

## 7. Back-Compat & Migration

- **Wire contract (`meshery.meshsync.core`)**: unchanged for Phase 1. No new/renamed fields on `KubernetesResource`; correlation is entirely a Server-side derived construct sitting beside the existing persisted rows.
- **Phase 2 wire addition**: MeshSync's builder-signal fields land inside `ComponentMetadata` (a pre-existing, already-loosely-typed `map[string]interface{}` column) under a new namespaced key, e.g. `ComponentMetadata["builderSignals"] = map[string]interface{}{"images": [...], "crdsPresent": [...]}` - additive to an already-open map, so old Server versions that don't understand the key simply ignore it (it round-trips through `utils.MergeMaps` untouched), and new Server versions work fine against old MeshSync versions that don't send it (`ExtractGroupingLabels`/grouping logic must treat its absence as "no builder signal available yet," never an error).
- **DB migration**: two new tables (`application_group_dbs`, `relationship_instance_dbs`), created via the same GORM auto-migrate path Server already uses for every other MeshModel registry table - no migration of *existing* data required; groups backfill lazily as future Add/Update events for already-discovered resources flow through `ProcessForGrouping` (Delete events don't need special backfill handling - see Risks).
- **Existing Connection auto-registration**: `getTypeOfConnection`'s fallback path is preserved verbatim so Grafana/Prometheus auto-registration behavior is byte-for-byte unchanged for clusters/objects that never resolve an `ApplicationGroup.identifiedAs` (e.g., before Phase 2 ships, or when a builder can't confidently identify the tool).

## 8. Risks / Failure Modes & Perf/Scale

- **Partial views / ordering**: an object can be persisted before its siblings arrive (ConfigMap before Deployment, or after a Server restart mid-resync). Mitigation: `FindOrCreateApplicationGroup` is idempotent and re-run on every Add/Update event for every member, not just once; a group converges to its final membership as objects trickle in, and `ProcessForGrouping` must be safe to call redundantly (no double-counting members, no duplicate `RelationshipInstance` rows - enforce via a DB unique constraint on `(relationshipDefinitionId, fromEntityId, toEntityId)`).
- **Label absence**: the majority of real-world clusters do **not** universally apply `app.kubernetes.io/*` labels. `ExtractGroupingLabels` must return `ok=false` (no group) rather than fabricate a key from, e.g., name-prefix heuristics - false groupings are worse than no grouping, since they'd corrupt the Connection/topology data derived from them. This is a hard MVP scope boundary: ungrouped resources continue to behave exactly as today (visible individually in the Kind-based dashboard, eligible for the existing substring-based auto-registration fallback).
- **Over-grouping**: `app.kubernetes.io/part-of` is sometimes set to something broad (e.g., `part-of: my-company`) shared across unrelated apps. Mitigation: prefer `/instance` (Helm-release-scoped, narrowest) over `/part-of` (broadest) in the tiering order specified above; log (structured MeshKit error/warn, not silent) when a group's member count crosses a sanity threshold (e.g., >200 resources) so operators can spot a mislabeled cluster; never auto-merge two groups discovered under different keys even if they later share members (that's a Phase-3 problem, not MVP).
- **Delete events and group shrinkage**: `meshsyncEventsAccumulator`'s `broker.Delete` case (`meshsync_events.go:242-247`) deletes the `KubernetesResource` row directly; `RelationshipInstance` rows referencing that `fromEntityId`/`toEntityId` become dangling unless cleaned up. Add an `ON DELETE CASCADE`-equivalent (GORM constraint or an explicit `DeleteRelationshipInstancesForEntity(id)` call) in the same Delete branch - this is exactly the kind of adjacent bug the maintainer mindset calls out: don't let a new feature introduce silent orphaned rows.
- **Perf/scale**: `ProcessForGrouping` running `go`-routine-per-event, unbounded, mirrors the existing `AutoRegistrationHelper` pattern's own known scaling ceiling (no worker pool, no backpressure) - acceptable for Phase 1 given the existing precedent, but flag in the PR description as a shared, pre-existing scaling limitation rather than a net-new one introduced by this feature. The grouping lookup query (`WHERE cluster_id=? AND key=... AND value=?` against `kubernetes_key_values`) needs a composite index on `(key, value, id)` or equivalent - verify `KubernetesKeyValue`'s existing `gorm:"primarykey"` composite (`ID, Kind, Key, Value` per `pkg/model/model.go:32-38`) already supports this access pattern efficiently before assuming a new index is required.
- **Builder-pattern false identification (Phase 2)**: image-tag heuristics (e.g., `bitnami/redis:7.2` -> Redis 7.2) are best-effort; `identifiedAs.confidence` and `evidence[]` must be part of the schema (already specified above) precisely so the UI can distinguish "confidently identified" from "guessed," and so a wrong guess doesn't silently poison the Connection type used for auto-registration - always allow the pre-existing substring fallback to win when confidence is low.

## 9. Test Plan (+ Runtime Verification)

- **schemas**: schema validation tests (`make validate-schemas`), `make consumer-audit` for the two new constructs; round-trip marshal/unmarshal tests for the generated Go models (standard for every construct in this repo).
- **meshkit**: unit tests for `ApplicationGroupFilter`/`RelationshipInstanceFilter` `Get`/`GetById` against an in-memory SQLite handler, mirroring `registry/registry_test.go`'s existing pattern.
- **meshery/server**:
  - Unit tests for `ExtractGroupingLabels` covering: no labels, only `/name`, only `/part-of`, `/instance` present (wins), multiple candidate labels with conflicting values (documents the precedence rule).
  - Unit tests for `ProcessForGrouping` idempotency: call twice with the same object, assert exactly one `ApplicationGroup` and no duplicate `RelationshipInstance` rows.
  - Integration test (extend `integration-tests/meshsync/database_content_assertion_testcases_test.go`'s existing pattern) - apply a fixture manifest with a labeled Deployment+Service+ConfigMap trio to a kind cluster, assert an `ApplicationGroup` with 3 members and a `sibling.matchlabels` `RelationshipInstance` between each pair materializes in the DB.
  - Regression test for `auto_register.go`'s fallback path: assert Grafana/Prometheus auto-registration still succeeds via substring match when no `ApplicationGroup.identifiedAs` exists.
  - Delete-path test: delete a grouped member, assert its `RelationshipInstance` rows are cleaned up and the group's member count decrements.
- **meshery/ui**: component tests for the new Applications table config (mirroring `resources/config.test.ts`'s existing shape) and the notification formatter extension (mirroring `meshsync_events.test.tsx`).
- **Runtime verification**: use the `verifier-meshsync` skill (`.claude/skills/verifier-meshsync/`) against a live kind cluster with a real Helm-installed app (e.g., `bitnami/redis` or `prometheus-community/kube-prometheus-stack`, both of which apply standard `app.kubernetes.io/*` labels) to confirm end-to-end: MeshSync publishes the labeled objects unchanged -> Server groups them -> the new REST endpoint returns the expected `ApplicationGroup` -> (Phase 2) `identifiedAs` resolves to the correct chart/tool+version.
- Per `CLAUDE.md`'s testing policy: every locally-runnable test above must actually be run (`make test` in each repo, plus the integration/runtime steps) before requesting review - none may be deferred to reviewers or follow-ups.

## 10. Effort & Phasing

**Phase 1 - MVP: label-based grouping (no version identification).**
- schemas: `application_group` + `relationship_instance` constructs. ~2-3 days incl. review.
- meshkit: two registry filters. ~1 day.
- server: `models/app_grouping.go`, the two new model files, the one-line accumulator hook, handlers, feature flag. ~4-5 days incl. tests.
- ui: Applications dashboard view + notification formatter. ~2-3 days.
- Total: roughly 2 sprint-weeks across repos, sequenced per Section 6.

**Phase 2 - full: builder-pattern fingerprinting (tool + version identification).**
- MeshSync: extend `GetProcessorInstance` with Deployment/Pod/CRD processors that extract image references and CRD-Kind presence into `ComponentMetadata["builderSignals"]` (mirrors `K8SService.Process` exactly - single-object, no new pipeline stage). ~3-4 days incl. tests.
- meshkit: shared image-reference-parsing utility if warranted by duplication between MeshSync and Server. ~1-2 days.
- server: `BuilderIdentifier` that runs over a group's accumulated `builderSignals` (from all current members) to resolve `{tool, version, confidence, evidence[]}`, triggered from the same `ProcessForGrouping` hook whenever a group's membership changes. ~5-7 days incl. a seed table of known image-pattern -> tool mappings and tests for ambiguous/low-confidence cases.
- ui: surface `identifiedAs` with confidence indicator in the Applications view and topology graph node styling (e.g., a recognizable icon once `identifiedAs.tool` resolves, echoing how `getConnectionDefinitions` already looks up per-tool styling from the component registry). ~2-3 days.
- Total: roughly 2-3 sprint-weeks, can start any time after Phase 1's schema lands (Section 6, step 6).

## 11. Open Questions

1. Should `RelationshipInstance.fromEntityId`/`toEntityId` be typed as `Uuid` even though `meshsyncmodel.KubernetesResource.ID` is currently a base64-encoded string (`SetID`, `pkg/model/model_converter.go:110-116`), not a UUID? Recommend keeping both as plain strings in the schema (not `$ref`ing the `Uuid` core type) to avoid a validation mismatch against MeshSync's actual ID format until/unless MeshSync's model migrates into `schemas` (tracked separately per `docs/agent-instructions/naming-conventions.md`).
2. Should the grouping key precedence (`instance` > `part-of` > `name`) be configurable per-cluster (e.g., some organizations may standardize on `/part-of` as their primary grouping label), or is a fixed precedence acceptable for MVP? Recommend fixed for Phase 1, revisit if early runtime verification against real customer clusters shows the fixed order produces poor groupings.
3. Should `ApplicationGroup` support cross-cluster membership (the same logical app deployed identically across two managed clusters) at all, or is `clusterId` a hard partition key forever? Recommend hard partition for both phases - cross-cluster application identity is a materially different (and larger) problem than this feature's scope.
4. Does the Connection state machine (`machines/kubernetes/*.go`) need a new state or transition to represent "Connection derived from an identified ApplicationGroup" versus today's `DISCOVERED` origin, or is tagging `ConnectionPayload.MetaData["applicationGroupId"]` sufficient? Recommend the metadata-only approach for Phase 1; revisit only if product requirements demand surfacing group provenance in the Connection's own state/status rather than its metadata.
5. Who owns curating the Phase-2 image-pattern -> tool/version mapping table long-term (a static seed file in `server/`, a MeshKit-owned shared table, or a MeshModel-registry-driven approach keyed off existing `ComponentDefinition`/model registrations)? Recommend starting with a MeshKit-owned static table (shared-logic-in-MeshKit per the ecosystem rule) with an explicit TODO to migrate to registry-driven matching once enough real-world signal exists to justify the added complexity.

---

**Key files referenced (all absolute paths):**

- `pkg/model/model_converter.go`, `pkg/model/process.go`, `pkg/model/preprocessor.go`, `pkg/model/model.go`
- `internal/pipeline/handlers.go`, `internal/output/{processor,broker}.go`
- `docs/design-spec_meshsync-infrastructure-synchronization.md`, `docs/agent-instructions/{architecture,naming-conventions,errors}.md`
- `schemas/schemas/constructs/v1beta3/relationship/relationship.yaml`, `schemas/models/v1beta3/relationship/relationship.go`
- `schemas/schemas/constructs/v1beta3/connection/connection.yaml`, `schemas/models/v1beta1/pattern/pattern.go`
- `schemas/docs/relationship-evaluation-engine-contract.md`
- `meshkit/models/meshmodel/registry/v1beta1/{component_filter,connection_filter}.go`, `meshkit/models/meshmodel/registry/v1alpha3/relationship_filter.go`
- `meshery/server/models/meshsync_events.go`, `meshery/server/models/meshsync_register_queue.go`
- `meshery/server/machines/helpers/auto_register.go`
- `meshery/server/policies/{policy,policy_hierarchical,policy_matchlabels}.go`
- `meshery/server/handlers/relationship_handlers.go`
- `meshery/ui/components/dashboard/resources/config.tsx`, `meshery/ui/components/layout/NotificationCenter/formatters/meshsync_events.tsx`
