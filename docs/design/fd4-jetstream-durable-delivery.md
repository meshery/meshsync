# Blueprint: Durable Delivery via NATS JetStream

## 1. Goal & Gap

**Goal.** Make MeshSync -> Meshery Server resource-event delivery at-least-once and durable: an event published while Meshery Server is disconnected must not be silently dropped; it must be redelivered once the consumer (or a fresh consumer identity, post-restart) reconnects.

**Gap (grounded).** MeshSync publishes over core NATS fire-and-forget:
- `internal/output/broker.go:19-32` (`BrokerWriter.Write`) calls `br.Publish(config.PublishTo, ...)` - `Publish` returns as soon as the local client hands the message to its outbound buffer; there is no server-side ack, no persistence, no consumer offset.
- `broker/nats/nats.go:143-167` (MeshKit `Nats.Publish`) is a straight `nc.Publish(subject, data)` - core NATS semantics: if no subscriber is connected to receive the message at publish time, the message is gone forever.
- `server/models/meshsync_events.go:126-131` subscribes with `SubscribeWithChannel("meshery.meshsync.core", "", out)` - again core NATS, no ack, no consumer state, no replay.
- Recovery today is **full re-list only**: `broker.ReSyncDiscoveryEntity` (`meshsync/handlers.go:203-204,256-258`) tears down and rebuilds every informer, republishing everything. There is no periodic/automatic trigger for this today - only an explicit request (`server/models/meshsync_events.go:378-392` `Resync()`) or a CRD-detected discovery-config change (`meshsync/handlers.go:444-457`). A future periodic-reconcile feature would call this on a timer; durability must be designed to *complement*, not replace, that belt-and-suspenders path.
- Adjacent interface gap: `broker.Handler` (`meshkit/broker/broker.go:12-15`) has no `Unsubscribe`. `ListenToRequests` (`meshsync/handlers.go:142-185`) registers one permanent `SubscribeWithChannel` for the process lifetime and never tears it down - not a per-call leak today, but it means neither MeshSync nor Server can ever cleanly release a subscription (durable JetStream consumers need exactly this to drain/rebind on config change or shutdown).

## 2. Current State Per Repo (grounded)

**MeshKit** (`meshkit`)
- `broker/broker.go:7-26` - `Handler` interface: `Publish`, `PublishWithChannel`, `Subscribe`, `SubscribeWithChannel`, `Info`, `DeepCopyObject`, `DeepCopyInto`, `IsEmpty`, `CloseConnection`, `ConnectedEndpoints`. No ack/nak, no `Unsubscribe`.
- `broker/nats/nats.go` - `Nats` struct wraps a single `*nats.Conn` (core NATS). `New(opts Options)` (line 41) sets `ReconnectWait`/`MaxReconnects`/handlers but never touches JetStream.
- `broker/channel/channel.go` - a second, in-process `Handler` implementation (`ChannelBrokerHandler`) used in tests/library mode; any interface change must keep this satisfying the interface too (it already stubs `Subscribe` as a no-op, line 154-163).
- `go.mod:30` already pins `github.com/nats-io/nats.go v1.47.0`, which ships the modern `github.com/nats-io/nats.go/jetstream` package - **no new external dependency needed** for JetStream client support.
- Error codes: `broker/nats/error.go` uses flat MeshKit-repo-wide codes (`meshkit-11118..11122`); `helpers/component_info.json:4` `next_error_code: 11327` - new codes are allocated by `make errorutil`, not hand-picked.

**MeshSync** (`meshsync`)
- `internal/output/broker.go` - `BrokerWriter` wraps a `broker.Handler` and calls `Publish` once per resource event; not JetStream-aware.
- `pkg/lib/meshsync/meshsync.go:333-350` (`createNatsBrokerHandler`) is the single construction point for the production broker connection - `nats.New(nats.Options{...})`.
- `pkg/model/model.go` + `pkg/model/model_converter.go:110-117` (`SetID`) - `KubernetesResource.ID` is derived from `base64(clusterID.Kind.APIVersion.Namespace.Name)`, a **stable per-object identity**, independent of Kubernetes UID or ResourceVersion. `KubernetesResourceMeta.ResourceVersion` (`model.go:47`) carries the per-object monotonic-within-object change marker from the API server. Together `(ID, ResourceVersion, EventType)` is the natural idempotency key - **not** UID (UID changes across delete/recreate of a same-named object, which is a case the ID formula deliberately treats as the *same* logical resource).
- `internal/output/inmemory_deduplicator*.go` - existing dedup is **file-output-only** (wired only in `meshsync.go:189-193`'s file-mode branch), keyed on Kubernetes UID, and is an in-memory, single-process, non-persistent cache - it has nothing to do with broker delivery and will not by itself solve JetStream redelivery dedup (different key, different lifecycle, wrong side of the wire).
- `meshsync/handlers.go:142-185` (`ListenToRequests`) - permanent `SubscribeWithChannel` for exec/log/resync/meta requests on `meshery.meshsync.request`-family subjects; separate from the `meshery.meshsync.core` publish path this feature targets, but shares the same `broker.Handler` and will need the same `Unsubscribe` capability for clean JetStream consumer teardown on resync/shutdown.
- No periodic reconcile timer exists yet (confirmed by absence of any `ticker`/cron in `meshsync/handlers.go`, `meshsync/meshsync.go`); resync is purely event-triggered.

**Meshery Server** (`meshery/server`)
- `models/meshsync_events.go` - `MeshsyncDataHandler.subscribeToMeshsyncEvents` (line 88) calls `ListenToMeshSyncEvents` (line 126) which does `mh.broker.SubscribeWithChannel("meshery.meshsync.core", "", out)` - core NATS, no ack.
- `meshsyncEventsAccumulator` (line 209-252) is **already partially idempotent by accident**: `Add` does `Create`, and on conflict falls back to `Updates` (lines 224-232) explicitly because "if MeshSync is restarted... the discovered data will have eventType as ADD, but the database would already have the data." `Update` does `Updates` (a GORM upsert-by-PK, safe to repeat). `Delete` does `Delete` (safe to repeat - 0 rows affected is not an error). This means **naive redelivery of the same event twice is already mostly safe today** - the design must preserve this property and make it airtight (no lost-update race between two concurrent redeliveries), not invent idempotency from scratch.
- `models/meshery_controllers.go:211-263` (`meshsynDataHandlersNatsBroker`) is where Server constructs its own `nats.New(nats.Options{...})` connection per Kubernetes context, using `brokerEndpoint` sourced from `controllerHandlers[MesheryBroker].GetPublicEndpoint()` - i.e., from the Broker CR's derived status endpoint. One `broker.Handler` per connected cluster context.
- `MeshsyncDataHandler.Resync()` (line 378-392) is the Server-side trigger for full re-list recovery, published to `MeshsyncRequestSubject = "meshery.meshsync.request"`.

**Meshery Operator** (`meshery-operator`)
- `api/v1alpha2/broker_types.go` (storage version, `+kubebuilder:storageversion` at line 91) and `api/v1alpha1/broker_types.go` (served, converted) both define `BrokerSpec{Version, Service BrokerServiceSpec, Size int32}` and `BrokerStatus{Endpoint Endpoint, Conditions}` - **no JetStream field today**.
- `pkg/broker/resources.go` hand-authors the NATS `StatefulSet`/`Service`/`ConfigMap`s (confirmed via `docs/proposals/broker-nats-direct-consumption.md:30-39`) - no JetStream file-store PVC, no `jetstream.enabled` config.
- `pkg/meshsync/meshsync.go:34-50` - `GetServerObject` injects `BROKER_URL` (+ optional `NATS_TOKEN` via secretKeyRef) into the MeshSync Deployment, sourced from `MeshSync.Status.PublishingTo`, itself copied from `Broker.Status.Endpoint.Internal` by `controllers/meshsync_controller.go` (`reconcileBrokerConfig`).
- **Existing, unmerged proposal** `docs/proposals/broker-nats-direct-consumption.md` already scopes JetStream as an **additive** `BrokerSpec.JetStream *JetStreamSpec{Enabled, Store, Size}` field (section 6.4) layered onto a chart-vendored NATS server, with NACK deferred to a later phase for JetStream *object* (Stream/Consumer) management via CRDs. This blueprint's Operator changes must slot into that plan rather than duplicate or contradict it.

**meshery/schemas** (`schemas`)
- Construct-based OpenAPI/YAML schema system (`schemas/constructs/<version>/<construct>/`) that generates **Server-side DB/API entity Go structs** (`models/<version>/<construct>/<construct>.go`) and TypeScript types - this is the source of truth for entities like `connection`, `environment`, `component` that Meshery Server persists and exposes over its REST API.
- **There is no `broker` construct today**, and the Broker/MeshSync **CRDs are not schemas-generated** - they are native kubebuilder types in `meshery-operator/api/v1alpha{1,2}` with their own `zz_generated.deepcopy.go` and CRD YAML under `config/crd/bases/`. This is an important distinction for this feature: the "schema-first" instruction in the ECOSYSTEM RULES applies to schemas' *actual* jurisdiction (Server-persisted entities and cross-repo wire contracts), not to Kubernetes CRD field additions, which are owned end-to-end by the Operator's own kubebuilder markers and conversion webhooks. Where this feature does touch schemas' jurisdiction is narrower than the prompt implies: it is the shape of any new **Server-side persisted delivery/consumer-offset state**, if the design introduces one (Phase 3 below argues it should not, in favor of JetStream's own consumer state, to avoid a second source of truth).

## 3. Proposed Architecture

### 3.1 Design principle: additive capability, not a competing interface

The `broker.Handler` interface stays exactly as-is for `Publish`/`Subscribe`/`PublishWithChannel`/`SubscribeWithChannel` - every existing consumer (MeshSync's `ListenToRequests`, Server's log/exec/store-update subscriptions, the file-mode `ChannelBrokerHandler`, adapters elsewhere in the ecosystem that MeshKit's AGENTS.md flags as fanning out broadly) keeps working unchanged. Durability is added as a **capability-negotiated extension**: a new, optional interface that a `broker.Handler` implementation may additionally satisfy, discovered via a type assertion at the call site - the same pattern Go's standard library uses for `io.ReaderFrom`/`io.WriterTo` optimizations. This avoids:
- Breaking the `broker.Handler` contract (adding a required method would break `ChannelBrokerHandler` and any other implementer, in-repo or downstream).
- Forcing every consumer that only wants at-most-once semantics (health pings, meta info, transient log streams) to deal with acks it doesn't need.
- A parallel "JetStreamHandler" type that call sites would have to choose between statically - callers keep one variable of type `broker.Handler` and opt into durability only where it matters (the `meshery.meshsync.core` publish path and its Server-side consumer).

### 3.2 New MeshKit interfaces (`broker/broker.go`)

```go
// DurablePublisher is an optional capability a broker.Handler implementation
// may additionally satisfy. Publish returns only after the broker has
// persisted the message and acknowledged the write (at-least-once, not
// fire-and-forget). idempotencyKey deduplicates redelivery-of-the-same-publish
// at the broker's own dedup window (JetStream's Msg-Id semantics).
type DurablePublisher interface {
    PublishDurable(subject string, message *Message, idempotencyKey string) error
}

// DurableSubscriber is an optional capability for a named, durable consumer:
// redelivery on crash/disconnect, explicit ack/nak, and Unsubscribe to
// release the consumer's server-side or client-side resources cleanly.
type DurableSubscriber interface {
    SubscribeDurable(subject, durableName string, handler func(*DurableMessage)) (Subscription, error)
}

// DurableMessage wraps Message with the ack/nak/term operations the consumer
// needs to signal outcome back to the broker.
type DurableMessage struct {
    *Message
    Ack     func() error
    Nak     func(delay time.Duration) error
    Term    func(reason string) error // permanent failure, do not redeliver
}

// Subscription is returned by SubscribeDurable and any future durable
// subscribe variant; it is the Unsubscribe handle broker.Handler itself
// lacks today.
type Subscription interface {
    Unsubscribe() error
    Drain() error // graceful: stop new deliveries, let in-flight acks complete
}
```

Additionally, close the adjacent interface gap called out in the prompt: add `Unsubscribe(subject, queue string) error` to the base `broker.Handler` interface (not just the durable path), since it is a real, standing gap that every consumer of core `Subscribe`/`SubscribeWithChannel` has today (`ListenToRequests` in MeshSync, three separate `SubscribeWithChannel` calls in Server's `meshsync_events.go`). This *is* a breaking interface addition, so it must ship as part of the same MeshKit release as the JetStream work (single coordinated bump), with both `Nats` and `ChannelBrokerHandler` updated to implement it in the same PR - see 4.1.

### 3.3 Streams, consumers, acks - concrete JetStream layout

- **Stream**: `MESHSYNC_EVENTS`, subjects `meshery.meshsync.core` (matches the existing wildcard-free literal subject MeshSync already publishes to - no subject-naming change). One stream per broker deployment (the Broker CR is per-managed-cluster per the Operator's topology - `meshery/meshery-operator`'s `MeshSync` CRD is one-per-cluster, so `MESHSYNC_EVENTS` is naturally scoped to that cluster's broker instance; no cross-cluster fan-in to reconcile).
- **Retention**: `WorkQueue`-adjacent but NOT `WorkQueuePolicy` (that policy deletes a message the instant *any* consumer acks it, which is wrong here - Server's durable consumer is the only consumer today, but `LimitsPolicy` with size/age caps is safer against future multi-consumer additions, e.g. an audit/event-sourcing consumer added later). Use **`LimitsPolicy`** with:
  - `MaxAge`: default 24h (long enough to survive a Server outage across a maintenance window, short enough to bound disk).
  - `MaxBytes`: default sized from a Broker CR field (3.5 below), defaulting to a small fixed cap (e.g. 256MiB) suitable for a single-cluster resource-event stream; exceeding it drops oldest (`Discard: DiscardOld`) rather than rejecting new publishes (`DiscardNew` would break MeshSync's publish path on storage pressure - the wrong failure mode for a fire-and-forget-turned-durable producer).
  - `Storage: FileStorage` (the whole point is surviving a broker pod restart, not just a Server disconnect - memory storage would defeat that).
  - `Replicas: 1` by default (single-NATS-node Broker CR topology today; make it a Broker CR field so a future clustered Broker can set 3).
- **Consumer**: one **durable, pull-based** consumer per Server instance, named deterministically from a stable identity Server already has - the `InstanceID` field already threaded through `MeshsyncDataHandler` (`server/models/meshsync_events.go:33,41-56`) - e.g. `durable := "meshery-server-" + instanceID.String()`. Pull (not push) so Server controls its own consumption rate and can batch-fetch after a reconnect instead of being flooded.
  - `AckPolicy: AckExplicit` - every message individually acked after the DB upsert succeeds, never acked speculatively before persistence (that would reintroduce at-most-once under a Server crash between receipt and DB write).
  - `AckWait`: 30s default (DB upsert should be fast; long enough to survive a slow Postgres write, short enough that a genuinely stuck consumer redelivers promptly).
  - `MaxDeliver`: bounded (default 10) with a **dead-letter subject** (`meshery.meshsync.core.dlq`) so a poison message (e.g. a payload that fails `Unmarshal` every time) doesn't loop forever - Server's `Unmarshal` (`meshsync_events.go:197-206`) already returns an error for bad payloads; on final-attempt failure, `Term()` the message rather than silently dropping it, and log-and-count DLQ arrivals for observability.
  - `DeliverPolicy: DeliverNew` on first-ever consumer creation (a brand-new Server instance should not replay the entire stream history - the full-resync path already handles "give me everything" semantics); `DeliverPolicy` is irrelevant on every subsequent reconnect because the **durable consumer's own stored ack floor** is what "resume from last acked" means - JetStream remembers per-durable-consumer position server-side, which is exactly the "resume-from-last-acked-after-downtime" behavior requested. This is why the consumer name must stay stable across Server restarts (tied to `InstanceID`, not a random UUID minted per process start).

### 3.4 Data flow

```
MeshSync pipeline handler (internal/pipeline/handlers.go)
        |  model.KubernetesResource, EventType
        v
internal/output.BrokerWriter.Write
        |  type-asserts br.(broker.DurablePublisher)
        |  builds idempotencyKey = obj.ID + "|" + obj.KubernetesResourceMeta.ResourceVersion + "|" + string(evtype)
        v
  [JetStream available?]
   yes -> br.PublishDurable(subject, msg, idempotencyKey)
            -> MeshKit nats/jetstream.go: js.PublishMsg(ctx, natsMsg-with-Nats-Msg-Id-header)
               -> NATS server: stream MESHSYNC_EVENTS stores msg, returns PubAck{Stream, Seq}
                  duplicate Msg-Id within the stream's dedup window -> broker
                  returns the ORIGINAL PubAck without re-storing (server-side idempotency,
                  not just client-side best-effort)
   no  -> fallback: br.Publish(subject, msg)  (existing core-NATS path, unchanged)
        v
NATS JetStream (file-backed, MESHSYNC_EVENTS stream)
        v
Meshery Server: durable pull consumer "meshery-server-<instanceID>"
        |  fetch batch, for each msg:
        v
MeshsyncDataHandler.meshsyncEventsAccumulator(event)
        |  same Create/fallback-Updates / Updates / Delete logic as today
        |  (already idempotent-by-accident; harden per 4.3)
        v
   success -> msg.Ack()
   transient DB error -> msg.Nak(backoff-delay)  (JetStream redelivers)
   permanent decode error, MaxDeliver exhausted -> msg.Term() + DLQ counter/log
```

### 3.5 Ordering, dedup/idempotency, at-least-once reality

- **Ordering**: JetStream preserves publish order **within a single stream for a single publisher connection**, and a pull consumer with `AckPolicy: AckExplicit` delivers in stream order by default (no explicit `Nak`-then-redeliver reordering unless a nak happens, which is expected and fine - GORM's `Updates`-by-PK make out-of-order Add/Update/Delete for *different* resources harmless, and same-resource events are already ordered per above). Do not enable parallel consumer instances (`num_pending` fan-out across multiple pull consumers) for this stream in v1 - a single logical Server consumer preserves the ordering guarantee the DB upsert logic implicitly depends on (Delete-then-stale-Add would be a problem; Add-then-Delete is not, since Delete is idempotent).
- **Dedup/idempotency - two layers, not one:**
  1. **Broker-side, publish-time**: JetStream's `Nats-Msg-Id` header + stream dedup window (`Duplicates` config, default matches `MaxAge`-scaled reasonable window, e.g. 2 minutes) catches **MeshSync-side retries of the same publish** (e.g. MeshSync's own client retries a publish after a transient network blip before getting the ack). This is what "tie to resourceVersion + existing content-dedup" in the prompt maps to: the idempotency key is `obj.ID + "|" + ResourceVersion + "|" + EventType`, which is stable for retries of the *identical* discovery event but distinct for a genuinely new change (ResourceVersion changes) - deliberately NOT reusing the file-mode `inmemory_deduplicator`'s UID-based key, since UID and this idempotency key serve different purposes (UID survives resourceVersion churn to prove liveness of the *same* API object; the JetStream key must change *with* resourceVersion so a real update isn't wrongly suppressed as a duplicate).
  2. **Consumer-side, delivery-time**: JetStream's redelivery (on Nak, AckWait timeout, or consumer crash-before-ack) can and will deliver the **same stream sequence number twice** - this is inherent to at-least-once and no Msg-Id dedup window prevents it (the message was already stored once; redelivery is not a re-publish). Server's DB upsert must be idempotent against this, which it already almost is (2.4 above); 4.3 hardens the remaining gap (a `Create`-then-concurrent-`Create` race is not actually possible here since Server processes one durable consumer sequentially, but a genuine "true upsert" - `ON CONFLICT DO UPDATE` - is worth doing anyway to remove the accidental-idempotency footgun of relying on error-driven fallback).
- **At-least-once vs. exactly-once, stated plainly**: this design delivers **at-least-once with an idempotent consumer**, which is operationally equivalent to exactly-once for this use case (DB state converges to the same result whether an event lands once or three times) but is **not** protocol-level exactly-once - do not claim exactly-once in docs or comments; claim "at-least-once, idempotent-consumer, net-effect-once."
- **Relationship to periodic reconcile (belt-and-suspenders)**: JetStream durability closes the "Server was down for N minutes" gap without a full re-list. A **future periodic-reconcile** feature (not built today - see 2, "no ticker exists") remains valuable as the second line of defense for failure modes JetStream does *not* cover: MeshSync itself missing a Kubernetes watch event (an API-server-side gap, upstream of the broker entirely - JetStream durability starts only once MeshSync has already observed and published the event), or stream data loss past `MaxAge`/`MaxBytes` retention, or an operator manually wiping the stream. The two features are complementary and this blueprint's stream retention sizing (3.3) should be tuned assuming periodic reconcile does NOT yet exist (so retention needs to cover realistic outage windows on its own) - once periodic reconcile ships, retention can likely be shortened since reconcile becomes the long-tail recovery path and JetStream only needs to cover short blips between reconcile ticks.

## 4. Per-Repo Changes

### 4.1 MeshKit (`meshkit`) - first, since everything downstream depends on it

**`broker/broker.go`** (modify)
- Add `Unsubscribe(subject, queue string) error` to `Handler` (breaking addition - see 6 for the coordinated-release requirement).
- Add `DurablePublisher`, `DurableSubscriber`, `DurableMessage`, `Subscription` types as defined in 3.2. These are new, separate interfaces - not additions to `Handler` - so they do not force `ChannelBrokerHandler` (or any downstream fake/mock implementing `broker.Handler`) to implement them; only `Unsubscribe` is a hard addition.

**`broker/nats/jetstream.go`** (new file)
- `type JetStreamOptions struct { StreamName string; Subjects []string; MaxAge time.Duration; MaxBytes int64; Replicas int; DurableName string; AckWait time.Duration; MaxDeliver int; DLQSubject string }`.
- `func (n *Nats) EnsureStream(ctx context.Context, opts JetStreamOptions) error` - idempotent stream create-or-update (`jetstream.New(nc)` then `js.CreateOrUpdateStream(ctx, streamConfig)`), called once at broker-handler construction time (not per-publish).
- `func (n *Nats) PublishDurable(subject string, message *broker.Message, idempotencyKey string) error` - marshals `message` (reuse the existing `json.Marshal` path from `Publish`), builds a `nats.Msg{Subject: subject, Data: data, Header: nats.Header{"Nats-Msg-Id": []string{idempotencyKey}}}`, calls `js.PublishMsg(ctx, msg)` with a bounded context timeout, wraps the returned `PubAck`/error into a new `ErrPublishDurable` (error code, see below).
- `func (n *Nats) SubscribeDurable(subject, durableName string, handler func(*broker.DurableMessage)) (broker.Subscription, error)` - `js.CreateOrUpdateConsumer` (pull, durable, explicit ack per 3.3), then a `Fetch`/`Messages()` iterator goroutine that wraps each `jetstream.Msg` into a `broker.DurableMessage{Message: parsed, Ack: msg.Ack, Nak: func(d) { msg.NakWithDelay(d) }, Term: func(reason) { msg.Term() }}` and invokes `handler`. Returns a `Subscription` wrapping the consumer's context-cancel + a `Drain()` that stops fetching and waits for in-flight handler calls (mirrors the existing `n.wg`/`n.ctx`/`n.cancel` shutdown pattern already in `Nats` at lines 33-37, 129-140 - reuse that WaitGroup discipline, don't invent a second one).
- `func (n *Nats) Unsubscribe(subject, queue string) error` - implements the new `Handler.Unsubscribe`; for the core (non-JetStream) path this needs `Nats` to track the `*nats.Subscription` handles `Subscribe`/`SubscribeWithChannel` currently discard (today `n.nc.QueueSubscribe(...)` return value is dropped at `nats.go:203,232` - fix that in the same change: store the subscription in a `map[string]*nats.Subscription` keyed by `subject+"|"+queue`, populated by `Subscribe`/`SubscribeWithChannel`, consumed and `.Unsubscribe()`'d by the new method). This is the direct fix for the "leaks MeshSync exec subscriptions" gap called out in the prompt - it was previously impossible to fix because there was nowhere to return the handle to.
- Capability check helper (optional, for callers): `func SupportsJetStream(h broker.Handler) (broker.DurablePublisher, bool)` - a tiny convenience wrapper over the type assertion so call sites (MeshSync's `BrokerWriter`) don't repeat assertion boilerplate. Not required, but consistent with MeshKit's role as the place this ergonomics lives.

**`broker/nats/error.go`** (modify)
- Add placeholder codes (`"replace_me"`, filled by `make errorutil` per the repo's own AGENTS.md convention) for: `ErrEnsureStreamCode`, `ErrPublishDurableCode`, `ErrCreateConsumerCode`, `ErrFetchMessagesCode`, `ErrUnsubscribeCode`, `ErrJetStreamUnavailableCode`. Run `make errorutil` before merge (per MeshKit's own `docs/agent-instructions/errors.md`), never hand-pick integers.

**`broker/channel/channel.go`** (modify - minimal)
- Add an `Unsubscribe(subject, queue string) error` method to satisfy the now-widened `Handler` interface (can be a real implementation given `ChannelBrokerHandler` already tracks `storage[subject][queue]` - closing and deleting the channel is a natural, correct `Unsubscribe`, and is strictly *more* correct than the current `CloseConnection`-only teardown). Do **not** implement `DurablePublisher`/`DurableSubscriber` on the channel broker - it is explicitly the non-durable, in-process/test broker; capability negotiation means callers that need durability simply won't get it from `ChannelBrokerHandler`, and that's the correct, intentional behavior (tests can assert `_, ok := br.(broker.DurablePublisher); !ok` for the channel broker as a regression guard against accidentally being asked to fake durability it can't provide).

**Tests** (new/modify, MeshKit `broker/nats/`)
- `jetstream_test.go`: stream creation is idempotent (create twice, no error, same config); `PublishDurable` with a repeated idempotency key against a real embedded/test NATS server (MeshKit likely already has NATS test-server bootstrap helpers used by existing broker tests - reuse, don't reinvent) returns the same `PubAck.Sequence` both times; `SubscribeDurable` receives redelivery after `Nak`; `Term` stops redelivery; `Unsubscribe` on a plain core subscription stops delivery (regression test for the leak fix).
- `broker_test.go` (or wherever `Handler` conformance is asserted today): add a compile-time assertion both `*Nats` and `*channel.ChannelBrokerHandler` still satisfy `broker.Handler` post-`Unsubscribe`-addition.

### 4.2 MeshSync (`meshsync`)

**`internal/output/broker.go`** (modify)
```go
func (s *BrokerWriter) Write(
	obj model.KubernetesResource,
	evtype broker.EventType,
	config config.PipelineConfig,
) error {
	msg := &broker.Message{
		ObjectType: broker.MeshSync,
		EventType:  evtype,
		Object:     obj,
	}
	if durable, ok := s.br.(broker.DurablePublisher); ok {
		key := idempotencyKey(obj, evtype)
		if err := durable.PublishDurable(config.PublishTo, msg, key); err != nil {
			if errors.Is(err, ErrJetStreamUnavailable) { // or a MeshKit-defined sentinel/typed check
				return s.br.Publish(config.PublishTo, msg) // fallback to core, unchanged path
			}
			return err
		}
		return nil
	}
	return s.br.Publish(config.PublishTo, msg) // unchanged core-NATS path
}

func idempotencyKey(obj model.KubernetesResource, evtype broker.EventType) string {
	rv := ""
	if obj.KubernetesResourceMeta != nil {
		rv = obj.KubernetesResourceMeta.ResourceVersion
	}
	return obj.ID + "|" + rv + "|" + string(evtype)
}
```
- The fallback-on-JetStream-unavailable path is the "(b) behavior when JetStream unavailable" requirement: MeshSync must not hard-fail publishing just because the stream/consumer isn't provisioned yet (e.g., an old Broker CR without JetStream enabled, or a rollout race where the Operator hasn't yet run `EnsureStream`). Define `ErrJetStreamUnavailable` as a sentinel MeshKit error (or a typed error with an `Unavailable() bool` method) returned by `PublishDurable` specifically for "stream/consumer not found" NATS JetStream error codes (`ErrStreamNotFound`, `ErrConsumerNotFound` from `nats-io/nats.go/jetstream`), distinct from a generic publish failure (which should propagate, not silently fall back and mask a real outage).

**`internal/output/error.go`** (new, or add to existing `internal/pipeline/error.go` if MeshSync's convention is per-package - confirm against `docs/agent-instructions/errors.md` allocation rule: MeshSync allocates per-package `error.go`, so `internal/output/` needs its own if it doesn't have one yet)
- Grep confirms `internal/output/` currently has **no** `error.go` - add one with the package's own code sequence (MeshSync's codes are flat small integers per package today, e.g. `"1000"`-range in `internal/config/error.go`, `"1001"`-`"1015"` in `internal/pipeline/error.go`, `"1004"`-`"1013"` in `meshsync/error.go` - these overlap across packages, consistent with the "unique per package" rule in CLAUDE.md, not globally unique). Add `ErrPublishDurableCode`, `ErrIdempotencyKeyCode` (if key construction can itself fail, likely not needed) as new package-local codes starting from whatever `internal/output` should start at (0 or 1, since it's a fresh file) - confirm against `helpers/component_info.json`'s `next_error_code` and let `make errorutil` (if MeshSync has adopted it) or manual allocation (if not yet) assign correctly; do not collide with `internal/pipeline`'s numbers since they're different packages, but do check if MeshSync's `errorutil` tooling treats the whole repo as one component (in which case codes ARE meant to be globally unique) - resolve this ambiguity by running `make errorutil-analyze`-equivalent before finalizing codes; this is a MUST-VERIFY-AT-IMPLEMENTATION item, flagged explicitly in Open Questions (11).

**`pkg/lib/meshsync/meshsync.go`** (modify `createNatsBrokerHandler`, lines 333-350)
- After `nats.New(...)` succeeds, call the new `EnsureStream` (4.1) with `JetStreamOptions` sourced from config (new `-jetstream`/env-driven flag or CRD field, wired the same way `BrokerURL` is today via `cfg.SetKey(config.BrokerURL, os.Getenv("BROKER_URL"))`). If `EnsureStream` fails (JetStream not enabled on this Broker, or insufficient permissions), log a warning and continue with the plain `broker.Handler` - **do not fail MeshSync startup** just because JetStream isn't available; this is the dual-mode/back-compat requirement (6, 7) made concrete at the exact call site.
- New CLI/env input: `-jetstream` bool flag (default `false` initially, flipped to `true` by default only after the Operator/Server rollout phases land - see 6) plus `-jetstreamStreamName`, mostly inherited from Broker-CR-injected env vars the same way `BROKER_URL` is today, so MeshSync doesn't need its own opinion about stream sizing - that's the Operator's/Broker-CR's job (4.4).

**`internal/config/types.go` / `default_config.go`** (modify)
- Add `JetStream bool` (or a richer `JetStreamConfig{Enabled bool; StreamName string}`) to whatever config surface carries `BrokerURL` today, so the CRD-config path (`crd_config.go`, `config.GetMeshsyncCRDConfigs`) can also toggle it per-cluster if the Operator injects it as a CR-derived setting rather than purely an env var.

**`meshsync/handlers.go`** (modify, minor - the `Unsubscribe` gap fix)
- `ListenToRequests` (line 142) should retain the returned subscription (`initRequestListener` currently discards whatever handle `SubscribeWithChannel` might someday return - line 180-184) so that a future graceful-shutdown path can call `Unsubscribe`. This is not strictly required for the JetStream feature to function, but it's the direct, low-risk fix for the adjacent gap the prompt calls out, and it's the same code path this feature is already touching for other reasons (best done together, flagged explicitly in the PR description per the CLAUDE.md "flag out-of-scope fixes" rule).

**Tests** (new)
- `internal/output/broker_test.go` (extend or create): `BrokerWriter.Write` calls `PublishDurable` when the injected `broker.Handler` fake implements `DurablePublisher`; falls back to `Publish` when it returns `ErrJetStreamUnavailable`; falls back to `Publish` when the fake doesn't implement `DurablePublisher` at all (today's behavior, must not regress). Table-test `idempotencyKey` for stability across repeated calls with the same `obj`/`evtype` and variance when `ResourceVersion` changes.
- `integration-tests/`: extend the existing broker-mode scenario file (`meshsync_as_binary_with_k8s_cluster_test_cases_mode_a_broker_test.go`) with a new case that starts MeshSync against a JetStream-enabled NATS (docker-compose already used for `integration-tests-setup` needs a JetStream-enabled config - a docker-compose/NATS-config change, see 9), publishes some events, kills the subscriber mid-stream, reconnects, and asserts zero events are lost (this is the direct test of the "broker-outage recovery" scenario called out in the prompt).

### 4.3 Meshery Server (`meshery/server`)

**`models/meshsync_events.go`** (modify)
- `ListenToMeshSyncEvents` (line 126) becomes durability-aware: type-assert `mh.broker.(broker.DurableSubscriber)`; if present, call `SubscribeDurable("meshery.meshsync.core", durableName, handler)` where `durableName := "meshery-server-" + mh.InstanceID.String()` (InstanceID already exists as a field, line 33); the handler function does exactly what `subscribeToMeshsyncEvents`'s `for range eventsChan` loop does today (call `meshsyncEventsAccumulator`), plus explicit ack/nak:
```go
func (mh *MeshsyncDataHandler) durableEventHandler(dm *broker.DurableMessage) {
	if dm.EventType == broker.ErrorEvent {
		// existing error handling, then Ack (it's not a retryable condition)
		dm.Ack()
		return
	}
	if err := mh.meshsyncEventsAccumulator(dm.Message); err != nil {
		mh.log.Error(err)
		dm.Nak(5 * time.Second) // transient (DB) failure: let JetStream redeliver
		return
	}
	dm.Ack()
}
```
  If the broker does NOT implement `DurableSubscriber` (older Broker CR without JetStream, or the `channel` broker in tests), fall back to today's `SubscribeWithChannel` path unchanged - same dual-mode principle as MeshSync's write side.
- `Stop()` (line 394-420) gains a call to `Unsubscribe`/`Drain` on whichever subscription handle `ListenToMeshSyncEvents` returns, using the new `broker.Subscription`/`Handler.Unsubscribe` - currently `Stop` only calls `mh.broker.CloseConnection()` (line 414-416), which is a blunt full-connection teardown; a clean `Drain()` first lets in-flight acks land before the connection closes, avoiding spurious redeliveries on graceful Server shutdown/restart (a real, if minor, resiliency improvement adjacent to this work).

**`meshsyncEventsAccumulator` idempotency hardening** (modify, lines 209-252)
- Replace the `Create` -> on-error -> `Updates` fallback for `broker.Add` with a genuine upsert: `mh.dbHandler.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoUpdates: clause.AssignmentColumns([...])}).Create(&obj)` (GORM's native `ON CONFLICT DO UPDATE`, available on both the Postgres and SQLite drivers MeshKit's `database/` package wraps). This removes reliance on "the first `Create` fails with a specific conflict error, so we retry with `Updates`" - which works today but is fragile (silently swallows *any* `Create` error, not just conflict errors, as a `Updates` retry) and becomes load-bearing rather than incidental once redelivery under JetStream makes duplicate-`Add` a routine, expected occurrence rather than a rare restart-timing coincidence. This is exactly the kind of "pay down technical debt encountered along the way" fix the maintainer mindset calls for: the current code already *needs* idempotent-Add semantics (its own comment says so), JetStream just makes the need load-bearing instead of incidental.
- `broker.Update` and `broker.Delete` are already idempotent (`Updates`-by-PK and `Delete`-by-PK are no-ops on a missing/already-deleted/already-current row) - no change needed there beyond confirming via a new test (below) that redelivery doesn't corrupt state or emit spurious errors.

**`models/meshery_controllers.go`** (modify, `meshsynDataHandlersNatsBroker`, lines 211-263)
- No change to the `nats.New(...)` call itself - JetStream is a capability on top of the same connection (`nats.Conn`), not a different connection type; MeshKit's `Nats.EnsureStream`/`PublishDurable`/`SubscribeDurable` all operate on the same `n.nc *nats.Conn` this function already produces. The only new consideration: if `EnsureStream`/consumer-creation should happen once per Broker (not once per Server-context-connection, since multiple Server processes/HA replicas might share one cluster context) - guard `EnsureStream` to be idempotent (create-or-update, per 4.1) so concurrent callers don't race destructively; JetStream's `CreateOrUpdateStream` is safe for this by design.

**Tests** (new/modify)
- `models/meshsync_events_test.go` (exists - extend): redelivery of an identical `broker.Add` event twice results in exactly one row, no error on the second delivery; redelivery of `broker.Delete` after the row is already gone returns success (already true, add as an explicit regression test); a fake `broker.Handler` implementing `DurableSubscriber` delivers a message, handler acks, `Nak` triggers on a forced accumulator error, dead-letter/`Term` path is exercised.

### 4.4 Meshery Operator (`meshery-operator`)

This slots directly into the existing `docs/proposals/broker-nats-direct-consumption.md` plan (section 6.4) rather than introducing a parallel design - align with it explicitly.

**`api/v1alpha2/broker_types.go`** (modify - storage version, additive)
```go
// JetStream enables durable, at-least-once event delivery on the broker.
// Additive: omitted/absent means JetStream is disabled (today's core-NATS
// behavior), preserving back-compat for every existing Broker CR.
// +optional
JetStream *JetStreamSpec `json:"jetStream,omitempty" yaml:"jetStream,omitempty"`

type JetStreamSpec struct {
	// Enabled turns on the JetStream file store and stream/consumer
	// provisioning for this broker.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// Storage selects file (durable across pod restarts) or memory
	// (faster, lost on restart - discouraged for this use case).
	// +kubebuilder:validation:Enum=file;memory
	// +kubebuilder:default=file
	Storage string `json:"storage,omitempty" yaml:"storage,omitempty"`

	// Size is the PVC size for file-backed JetStream storage.
	// +optional
	Size resource.Quantity `json:"size,omitempty" yaml:"size,omitempty"`

	// MaxAge bounds how long a message is retained regardless of ack
	// state (survives Server outages up to this window).
	// +kubebuilder:default="24h"
	MaxAge metav1.Duration `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`

	// Replicas is the JetStream replica count for the stream (requires
	// a clustered NATS deployment; 1 for the current single-node Broker).
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty" yaml:"replicas,omitempty"`
}
```
- Mirror the identical additive field into `api/v1alpha1/broker_types.go` (served, non-storage) plus the conversion functions in `api/v1alpha1/conversion.go` (both directions - `ConvertTo`/`ConvertFrom` the storage `v1alpha2` type), consistent with how the existing `Service BrokerServiceSpec` field is already dual-versioned.
- Regenerate `config/crd/bases/meshery.io_brokers.yaml` (controller-gen) and `zz_generated.deepcopy.go` for both API versions.

**`pkg/broker/resources.go` / `pkg/broker/broker.go`** (modify)
- When `Spec.JetStream != nil && Spec.JetStream.Enabled`, render the StatefulSet with `-js` server flags (or the equivalent `nats.conf` `jetstream { store_dir: ..., max_file: ... }` block the ConfigMap already generates per `resources.go:69`) and add a `VolumeClaimTemplate` sized from `Spec.JetStream.Size`. If the operator has already moved to the chart-vendored-manifests approach from the existing proposal by the time this lands, this becomes a values overlay (`config.jetstream.enabled`, `config.jetstream.fileStore.pvc.size`) instead of hand-authored YAML mutation - either way, the JetStream toggle is a `BrokerSpec`-driven overlay on top of whatever mechanism 4.4's sibling proposal has landed, not a separate code path.
- `controllers/broker_controller.go` needs `Owns(&corev1.PersistentVolumeClaim{})` added to its watch set when JetStream is enabled (mirrors the "Add `Owns(...)` for any additional chart-owned kinds" note already in the existing proposal, section 6.3) so PVC changes/GC are reconciled.

**Stream/consumer provisioning - who calls `EnsureStream`?**
- The Operator does **not** call `EnsureStream` itself (it has no NATS client dependency and the existing proposal is explicit about not re-adding heavyweight runtime deps). Stream/consumer creation is MeshSync's and Server's job at their own connection-construction time (4.2, 4.3), guarded to be idempotent. The Operator's only responsibility is: (a) turn on JetStream at the **server** level (file storage enabled, PVC provisioned) via `BrokerSpec.JetStream`, and (b) surface enough config (stream name convention, e.g. always `MESHSYNC_EVENTS`, no need to make this configurable initially) that MeshSync/Server agree on it without a new wire contract. This keeps the Operator's surface small and avoids a second place that needs NATS/JetStream client code.

**Tests**
- `pkg/broker/broker_test.go` / `resources_test.go`: `JetStream.Enabled` produces the expected StatefulSet JetStream flags/ConfigMap block and PVC; `JetStream == nil` (or `Enabled: false`) produces byte-identical output to today (explicit back-compat regression test).
- `controllers/broker_controller_test.go` (envtest): PVC is created/owned/GC'd when JetStream is enabled; toggling `Enabled` false->true reconciles in place without pod deletion (or documents that it requires a rolling restart if JetStream can't be hot-enabled on a running NATS process - verify against NATS server capability; likely requires a restart, which is acceptable and should be called out in the CRD's field doc comment).

## 5. Schema/CRD Changes

**Where "schema-first" actually applies here**: the Broker/MeshSync CRDs are **not** `meshery/schemas`-sourced (see 2, "meshery/schemas" current state) - they are native kubebuilder types owned end-to-end by `meshery-operator`. The CLAUDE.md/ecosystem-rule instruction to start schema changes in `meshery/schemas` applies to constructs schemas actually owns (Server-persisted entities, cross-repo wire/API contracts). Applying it literally to a Kubernetes CRD field would be a **category error** - CRDs are versioned and converted via kubebuilder's own conversion-webhook machinery (`api/v1alpha1/conversion.go`), which is a different, already-established versioning discipline than schemas' OpenAPI-YAML-to-Go-struct pipeline. This blueprint therefore treats:
- **`BrokerSpec.JetStream` (CRD field)**: owned and versioned entirely within `meshery-operator` (4.4), following its existing `v1alpha1`<->`v1alpha2` conversion pattern - no `meshery/schemas` PR needed.
- **`broker.Message`/`broker.DurableMessage` (Go wire types in MeshKit)**: these are also not `meshery/schemas` constructs - they are MeshKit-native (`broker/messaging.go` already defines `Message`, `ObjectType`, `EventType` locally, not sourced from schemas), and this feature only adds fields/wrapper types alongside them, following the existing pattern rather than diverging from it.
- **Where schemas genuinely could be touched, and isn't, by design**: if this design had introduced a Server-persisted "delivery cursor" or "last-processed-sequence" table as new durable state, THAT would need a schemas construct (it would be a new persisted entity). This blueprint deliberately avoids that (3.5's "resume-from-last-acked" relies on JetStream's own server-side consumer-ack-floor, not a second Server-side ledger) specifically to avoid creating a second source of truth that would need schemas coordination and its own migration story. This is called out explicitly as an architecture decision, not an oversight: **do not add a Server-DB delivery-cursor table**; JetStream is authoritative for "what has been acked."

**Versioning/back-compat for the one real schema-adjacent surface**: none required beyond what 4.4 already covers (additive CRD field, dual-versioned per the Operator's existing pattern). If a future iteration needs a schemas-owned "Broker connection" entity (e.g., if Server ever persists broker-connection metadata as a first-class API resource, which it does not do today - Server holds broker connections in-memory per `MesheryControllersHelper`, not in a DB table), that would be a separate, follow-on schemas change modeled on `schemas/constructs/v1beta1/connection/connection.yaml` (the AGENTS.md-designated canonical reference) - flagged here as a non-blocking future item, not part of this feature's critical path.

## 6. Cross-Repo Sequencing & Feature-Flagging

Strict dependency order, each phase independently mergeable and releasable:

1. **MeshKit first** (4.1): ship `Unsubscribe` + `DurablePublisher`/`DurableSubscriber`/`jetstream.go` in one MeshKit release. This is a `go.mod` version bump every downstream consumer must eventually pick up, but because `Unsubscribe` is the only breaking addition to `Handler` and both in-repo implementations (`Nats`, `ChannelBrokerHandler`) gain it in the same PR, MeshKit's own CI is green immediately - the breaking change is contained to MeshKit's release, not felt by consumers until *they* upgrade their `go.mod` pin, which they do explicitly in the next steps (Go's module system means old MeshSync/Server binaries pinned to the prior MeshKit version keep compiling and running against old MeshKit unchanged - this is not a runtime break, it is a compile-time interface addition that only bites when a consumer bumps its `go.mod`).
2. **Meshery Operator** (4.4): add `BrokerSpec.JetStream` (CRD field only - no MeshKit dependency yet, this is pure Go-type/CRD-YAML work) and the server-side JetStream-enablement rendering. Ship with `Enabled: false` as the default - existing Broker CRs and fresh installs alike get core-NATS-only behavior until an operator/admin opts in. This can and should merge **before** MeshSync/Server even bump their MeshKit pin, since it only changes what the NATS *server* can do, not how anyone talks to it.
3. **MeshSync + Meshery Server, together** (4.2 + 4.3): both bump their MeshKit `go.mod` pin to the new release, both gain the capability-negotiated durable publish/subscribe paths, both default to **using JetStream automatically whenever the connected broker's stream/consumer creation succeeds** (no separate MeshSync-side or Server-side feature flag needed beyond "does `EnsureStream`/`SubscribeDurable` succeed" - the capability negotiation *is* the feature flag, driven entirely by whether the Broker CR has JetStream enabled). This is deliberately not a `--enable-jetstream` CLI flag on MeshSync/Server that an admin must separately flip - that would create a three-way flag matrix (Broker JetStream on/off x MeshSync flag on/off x Server flag on/off) that's harder to reason about and easier to misconfigure than "MeshSync/Server always try, and gracefully no-op back to core NATS if the broker doesn't support it."
4. **Dual-mode is therefore not a separate rollout phase - it is the steady-state design**: a fleet can have some Broker CRs with `JetStream.Enabled: true` and others without, indefinitely, and every MeshSync/Server instance handles both without per-instance configuration. This is the single most important sequencing decision: **it eliminates the need for a synchronized "flip the flag everywhere" cutover**, which is usually the riskiest part of a durability migration. Operators upgrade Broker CRs to enable JetStream cluster-by-cluster, at their own pace, with zero coordination required with MeshSync/Server upgrade timing (as long as MeshSync/Server are already on a MeshKit version that *understands* JetStream capability negotiation - which they should be, from step 3 onward, regardless of whether any given Broker has JetStream turned on yet).

## 7. Back-Compat & Migration

- **Old MeshSync + old Server, new Broker (JetStream enabled)**: MeshSync/Server on pre-upgrade MeshKit still call plain `Publish`/`SubscribeWithChannel` against core NATS - JetStream-enabled NATS servers still fully support core pub/sub on the same subjects (JetStream is additive at the server level too; a stream merely *also* captures messages published to its subjects, it doesn't require publishers to use the JetStream API). **Zero behavior change** for old binaries talking to a JetStream-upgraded broker - this is the critical compatibility property that makes the Operator-first sequencing (step 2 above) safe to ship independently.
- **New MeshSync/Server, old Broker (JetStream disabled/absent)**: `EnsureStream`/`SubscribeDurable` fail with `ErrJetStreamUnavailable` (or the underlying NATS "JetStream not enabled" error), both sides fall back to the unchanged core `Publish`/`SubscribeWithChannel` path. **Zero behavior change** versus today.
- **New MeshSync, new Broker, old Server** (partial upgrade mid-rollout): MeshSync publishes durably into the stream; old Server subscribes with plain core `SubscribeWithChannel` on the same subject - NATS delivers core-subscribed messages **in addition to** stream-captured ones (they're not mutually exclusive), so old Server keeps receiving events exactly as before, just without the durability benefit until it, too, upgrades. No message is double-processed by an old Server (it never sees stream redelivery, since it never asks for JetStream consumption).
- **Migration is therefore per-cluster and per-component-independent** - no dual-write, no shadow-stream-then-cutover dance needed, no data migration (there is no prior persisted state to migrate; the stream starts empty on first enablement and simply begins capturing new events from that point forward).
- **Rollback**: disabling `BrokerSpec.JetStream.Enabled` on an already-upgraded Broker reverts the server to core-NATS-only; MeshSync/Server (any version) fall back to their core paths automatically (the capability check fails), no code rollback required on MeshSync/Server, just a CRD edit (though the NATS StatefulSet likely needs a restart to actually drop JetStream file storage - fine, since disabling durability is an intentional, rare operator action, not a hot path).

## 8. Risks / Failure Modes & Perf/Scale

- **Storage exhaustion**: `MaxBytes`/`MaxAge` + `DiscardOld` bound worst case to "silently lose the oldest still-unacked events once the cap is hit," which is a **regression path back toward at-most-once** for the oldest events specifically, not a crash or a broker outage. Mitigate: expose Prometheus-style stream metrics (JetStream natively reports `num_pending`, `bytes`, `messages` per stream via its API - Server or a sidecar should scrape and alert when `num_pending` grows unboundedly, which signals "Server has stopped acking," the actual root cause worth alerting on rather than the storage symptom). Size defaults conservatively (3.3) and make both fields Broker-CR-configurable so large clusters with high churn can size up.
- **Redelivery storms**: a poison message that always fails DB upsert (e.g., a schema-incompatible payload from a version-skewed MeshSync) redelivers up to `MaxDeliver` times, then `Term()`s to the DLQ subject - bounded, not infinite. A slow-but-not-failing Server (DB under load) risks `AckWait` timeouts causing *spurious* redelivery-while-still-processing; set `AckWait` generously (30s, per 3.3) and consider `InProgress` acks (JetStream supports `msg.InProgress()` to reset the ack timer without acking, useful if a batch DB write can occasionally run long) as a follow-up if 30s proves too tight under real load - flagged as a tuning parameter to watch, not a blocking design gap.
- **Ordering under redelivery**: covered in 3.5 - single-consumer-per-Server-instance avoids cross-consumer reordering; a `Nak`-triggered redelivery of message N while messages N+1..N+k have already been acked is possible and is **fine** given the GORM upsert-by-PK semantics (out-of-order Add/Update is a no-op-safe overwrite, not a corruption), but a `Nak` of a `Delete` while a later `Add` for the *same resource ID* has already been acked (same-resource reordering) would be a real bug - this can only happen if the accumulator's own error path naks a `Delete` due to a transient DB error while a subsequent event for the same object races ahead; mitigate by keeping the pull-consumer single-threaded/sequential per Server instance (do not parallelize `Fetch`-loop processing across goroutines in 4.3's implementation) so same-stream-order is preserved end-to-end into the DB calls.
- **Multiple Server replicas / HA**: if Meshery Server ever runs as more than one replica against the same cluster context (not confirmed as current architecture, but worth flagging), a stable `durableName` derived from `InstanceID` must mean "one logical Server identity," not "one process" - if `InstanceID` is per-process rather than per-logical-deployment, two replicas would each create their own durable consumer and **each get a full copy of every event** (JetStream durable consumers are independent unless explicitly made into a queue group). Verify `InstanceID`'s actual scope before finalizing `durableName` (Open Question, 11) - if it turns out to be per-replica, use a JetStream **pull consumer with a shared durable name in a queue-group-like pattern** (JetStream doesn't have NATS core's queue groups natively for pull consumers in the same way, but multiple pull-fetchers against the *same* durable consumer name safely share the workload - confirm this is architecturally what's wanted before assuming single-replica).
- **Performance**: `PublishDurable` is slower than `Publish` per-message (round-trip ack instead of fire-and-forget) - for MeshSync's typical burst-during-initial-discovery pattern (potentially hundreds of resources published in a tight loop at startup/resync), this could meaningfully slow down the initial-sync burst. Mitigate with JetStream's **async publish** API (`js.PublishMsgAsync`, which still gets acked but doesn't block the caller waiting for each individual ack - batches multiple in-flight acks and surfaces errors via a future/channel) as the actual `PublishDurable` implementation detail, not a naive synchronous per-call round trip; this is an implementation-level optimization within 4.1's `jetstream.go` worth specifying explicitly rather than leaving to chance, since a synchronous-per-message implementation would be a real regression during MeshSync's burst-discovery phase.

## 9. Test Plan (+ Runtime Verification)

**Unit** (all repos, `make test` equivalent, must pass before review per CLAUDE.md):
- MeshKit: stream idempotent creation, publish-dedup-by-Msg-Id, ack/nak/term behavior, `Unsubscribe` on core subscriptions, `ChannelBrokerHandler`'s new `Unsubscribe`, interface-satisfaction compile checks.
- MeshSync: `BrokerWriter` capability-negotiation branches (durable/fallback/plain), `idempotencyKey` stability/variance table tests.
- Meshery Server: idempotent-upsert-on-redelivery for Add/Update/Delete, durable-vs-fallback subscribe branches, Nak-on-transient-error / Term-on-permanent-error paths.
- Meshery Operator: `BrokerSpec.JetStream` additive-field back-compat (nil/false produces byte-identical manifests to today), PVC rendering when enabled, v1alpha1<->v1alpha2 conversion round-trip for the new field.

**Integration** (`make integration-tests` in MeshSync; envtest in Operator):
- MeshSync's `integration-tests-setup` NATS docker-compose needs a JetStream-enabled config variant (new compose file or a `-js` flag on the existing NATS container invocation) - add alongside the existing setup, not replacing it, so both JetStream-off and JetStream-on integration runs exist.
- New scenario: publish N events, kill the Server-side (or test-harness) subscriber process mid-stream, wait, restart the subscriber with the **same durable name**, assert all N events are eventually received exactly-once-in-effect (some may be redelivered at the transport level if the kill happened before an ack, but the **DB/observed-count assertion is "N distinct resources present," not "N messages received,"** matching the idempotent-consumer contract from 3.5).
- Operator envtest: JetStream toggle reconciles PVC/StatefulSet correctly; toggling on an existing Broker doesn't orphan the old (non-JetStream) StatefulSet.

**Runtime verification** (the prompt explicitly calls for broker-outage recovery, and the current `verifier-meshsync` skill explicitly does not cover broker mode - this is a real gap to close, not an oversight to note and skip):
- Extend `.claude/skills/verifier-meshsync/` (or add a sibling skill/script) to stand up a JetStream-enabled NATS alongside the kind cluster, run MeshSync in **broker mode** (not file mode, which the skill currently defaults to specifically because it avoids needing a broker) pointed at it, and a minimal subscriber harness (could be a small Go test binary using MeshKit's own `nats` package with `SubscribeDurable`) standing in for Server.
- Drive the broker-outage scenario concretely: start MeshSync + subscriber, create K resources (observe K published+acked), **stop the subscriber process** (simulating Server down), create/modify M more resources while it's down (MeshSync keeps publishing durably - assert via NATS's own `nats stream info MESHSYNC_EVENTS` CLI or the JetStream API that `num_pending` grows by M), **restart the subscriber with the same durable name**, assert it drains exactly the M pending messages without needing a MeshSync-side resync, and that total observed-distinct-resources equals K+M.
- This is the single most important net-new verification artifact this feature needs, since it's the concrete, falsifiable version of the feature's core promise ("events are PERSISTED in the broker if connectivity breaks") - a passing unit/integration test suite alone does not prove this against a real broker process, per the maintainer-mindset instruction to actually run runtime verification rather than defer it.

## 10. Effort & Phasing

**MVP (durable core stream + idempotent consumer)** - the minimum slice that delivers the actual promise:
- MeshKit: `Unsubscribe` + `DurablePublisher`/`SubscribeDurable` + `jetstream.go` (stream/consumer create, publish-with-msg-id, fetch-with-ack/nak/term). No async-publish optimization yet (ship synchronous first, correct-but-slower, optimize in a fast-follow once the burst-publish perf risk (8) is actually measured).
- MeshSync: `BrokerWriter` capability negotiation + idempotency key + `EnsureStream` at connection time + fallback-on-unavailable.
- Meshery Server: durable subscribe + Ack/Nak wiring + the GORM upsert hardening (this last part should ship regardless of JetStream, as a standalone idempotency fix, since it's valuable independent of this feature - flag it as such in the PR).
- Meshery Operator: `BrokerSpec.JetStream` additive field + file-storage StatefulSet/PVC rendering, default `Enabled: false`.
- Runtime verification: the broker-outage-recovery scripted scenario (9).
- **Explicitly deferred out of MVP**: DLQ consumption/alerting tooling (the DLQ subject exists and messages land there, but building an operator-facing "view DLQ contents" UX is separate); async publish optimization; multi-replica-Server queue-group consumer pattern (until confirmed necessary, per Open Question below); JetStream stream/consumer management via NACK CRDs (explicitly deferred in the existing Operator proposal too - this feature does not need NACK, since MeshSync/Server provision their own stream/consumer directly via the NATS client, which is simpler and sufficient at this scale).

**Full feature** (fast-follow after MVP is running in production on at least one real cluster):
- Async publish for burst performance.
- Stream/consumer metrics surfaced through Server's existing observability (Prometheus) with alerting on `num_pending` growth and DLQ arrival rate.
- Multi-Server-replica consumer pattern, if confirmed needed.
- Broker-CR-configurable `MaxBytes`/`MaxAge`/`Replicas` exposed through Meshery's UI/CLI (today they'd be CRD-YAML-edit-only, which is fine for MVP but not for a polished admin experience).
- Coordination with periodic-reconcile once that feature is designed/built, to retune retention windows per 3.5's closing note.

**Rough sizing**: MeshKit interface + JetStream client work is the largest single chunk (new file, new error codes, careful ack-lifecycle correctness, must not regress the widely-fanned-out `broker.Handler` contract) - call it the anchor task the rest sequences behind. MeshSync and Server changes are each comparatively small and mostly mechanical once MeshKit's capability interfaces exist. Operator's CRD-field addition is small and independent. The runtime-verification skill extension is nontrivial (new broker-mode harness where only file-mode exists today) and should not be shortchanged, since it's the only artifact that actually falsifies the feature's core durability claim against a real process.

## 11. Open Questions

1. **`InstanceID` scope** (risk item, 8): is it stable per logical Server deployment or minted per-process/per-restart? This directly determines whether `durableName` derived from it produces "one consumer across restarts" (correct, required) or "a new orphaned consumer every restart" (wrong, would leak JetStream consumers and never resume from an ack floor). Must be confirmed in `meshery/server` before finalizing 4.3's `durableName` derivation.
2. **Server HA / multi-replica**: does any current or near-term deployment topology run more than one Server process against the same cluster context concurrently? Determines whether the queue-group-like shared-durable-consumer pattern (8) is needed in MVP or safely deferred.
3. **MeshSync error-code allocation scope** (flagged in 4.2): is MeshSync's `errorutil`-style code allocation per-package (as CLAUDE.md's "unique per package" phrasing and the observed overlapping `"100X"` ranges across `internal/config`/`internal/pipeline`/`meshsync` suggest) or whole-repo? This determines the exact starting integers for `internal/output/error.go`'s new codes and must be resolved by running whatever `errorutil`-equivalent tooling MeshSync has adopted (or manual cross-file dedup) before merge, not assumed from this analysis alone.
4. **JetStream hot-enable on a running NATS StatefulSet**: can `BrokerSpec.JetStream.Enabled` flip `false`->`true` in place, or does the NATS server require a restart to pick up `-js`/`jetstream{}` config? Confirm against the NATS server's actual reload behavior (the config-reloader sidecar already in `pkg/broker/resources.go` handles *some* hot-reloads today, but JetStream enablement may not be one of them) and document the answer in the CRD field's godoc rather than leaving it implicit.
5. **Interaction with the in-flight `broker-nats-direct-consumption.md` proposal**: that proposal's Phase B (section 10) already scopes `BrokerSpec.JetStream` + file-store PVC as its own deliverable, on its own timeline, independent of this feature request. Confirm with the Operator maintainer whether this blueprint's 4.4 should be implemented as literally the same PR/phase as that proposal's Phase B (avoiding two competing designs for the same field) or as a distinct, coordinated-but-separate change that lands the identical `BrokerSpec.JetStream` shape.
6. **DLQ consumption ownership**: once messages land on `meshery.meshsync.core.dlq`, who/what consumes, surfaces, or alerts on them? This blueprint provisions the subject and the `Term()`-on-exhaustion behavior but explicitly defers building consumption tooling to a fast-follow (10) - confirm that's acceptable for MVP or whether even minimal DLQ visibility (a log-line counter, say) is a hard MVP requirement.
7. **Stream naming/subject scope if MeshSync ever multiplexes multiple pipelines onto different subjects**: today all resource pipelines share `meshery.meshsync.core` (confirmed, 2), so one stream/one subject is sufficient; if a future change splits publishing across multiple subjects (as `logs`/`exec` already do, though those are explicitly out of scope for *this* durability feature per the prompt's framing), confirm the stream's `Subjects` list should be widened rather than adding parallel streams, to keep ordering/dedup reasoning in one place.

---

**Key files referenced (all absolute paths):**

- `meshkit/broker/broker.go`, `meshkit/broker/messaging.go`, `meshkit/broker/nats/nats.go`, `meshkit/broker/nats/error.go`, `meshkit/broker/channel/channel.go`, `meshkit/go.mod`, `meshkit/helpers/component_info.json`
- `internal/output/broker.go`, `internal/output/inmemory_deduplicator.go`, `internal/output/inmemory_deduplicator_streaming.go`, `pkg/lib/meshsync/meshsync.go`, `pkg/model/model.go`, `pkg/model/model_converter.go`, `meshsync/handlers.go`, `meshsync/meshsync.go`, `meshsync/error.go`, `internal/config/types.go`, `internal/config/default_config.go`, `internal/pipeline/error.go`, `.claude/skills/verifier-meshsync/SKILL.md`, `integration-tests/meshsync_as_binary_with_k8s_cluster_test_cases_mode_a_broker_test.go`
- `meshery/server/models/meshsync_events.go`, `meshery/server/models/meshery_controllers.go`
- `meshery-operator/api/v1alpha1/broker_types.go`, `meshery-operator/api/v1alpha2/broker_types.go`, `meshery-operator/pkg/meshsync/meshsync.go`, `meshery-operator/docs/proposals/broker-nats-direct-consumption.md`
- `schemas/AGENTS.md`, `schemas/schemas/constructs/v1beta1/connection/connection.yaml`
