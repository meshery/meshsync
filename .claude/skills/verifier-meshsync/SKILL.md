---
name: verifier-meshsync
description: Runtime-verify a MeshSync change by running the agent against a local Kubernetes cluster and observing its behavior in the debug log and file output. Use this whenever verifying, testing, or confirming a MeshSync code change actually works at runtime - CRD watching and informer resync, resource discovery/publishing, filtering (namespaces/resources), whitelist/blacklist config, backoff/reconnect - and whenever /verify runs on the meshsync repo. Reach for it instead of a cold start any time you need to see MeshSync's discovery pipeline actually execute against a cluster, not just pass tests.
---

# Verifying MeshSync at runtime

MeshSync is a Kubernetes agent: it watches the cluster with dynamic informers and
publishes resource events to a NATS broker (or to a file). **The surface is the
running agent against a real cluster** - a change is verified by making the
changed code path execute and reading what the agent logs and writes, not by
running its unit tests (CI does that).

The whole game is: get the agent running against a local cluster, drive the
cluster so the changed code path fires, and read the evidence out of the debug
log. This skill encodes the parts that are fiddly to rediscover each time.

## Quick start

```bash
SKILL=.claude/skills/verifier-meshsync/scripts/verify-env.sh
$SKILL up      # pin an isolated kubeconfig to a reachable local cluster + install CRD/CR
$SKILL run     # build the binary and launch it in file mode with DEBUG logging
sleep 10       # let it connect and finish initial discovery
tail -f "$($SKILL logs)"   # watch it; drive events in another shell (see below)
$SKILL down    # stop meshsync, remove the CRD/CR/namespace it added
```

`verify-env.sh` only automates the deterministic setup. **Driving the surface and
choosing what to observe is your job** - it depends on what changed.

## Why each setup step matters

Read these before trusting the harness; they are the non-obvious traps.

- **Don't trust the default kubeconfig context.** In practice it often points at
  an unreachable remote cluster, and `mesherykube.New(nil)` will silently try to
  use it. `up` probes local contexts (docker-desktop, minikube, kind, orbstack,
  ...) for `/readyz` and writes a *minified, isolated* kubeconfig so the binary
  cannot wander off. Pass `MESHSYNC_VERIFY_CONTEXT=<name>` to force one.
- **`WatchCRDs` only runs when the MeshSync CR is present.** `useCRDFlag` (in
  `pkg/lib/meshsync/meshsync.go`) is true only when the `meshsyncs.meshery.io`
  CRD *and* the `meshery-meshsync` CR (namespace `meshery`) exist. Without them
  the CRD-watch goroutine never starts and you will "verify" nothing. `up`
  installs both (meshery-operator CRDs + `integration-tests/infrastructure/meshsync.yaml`).
- **Use file output mode, not broker mode.** `--output file` runs the full
  discovery pipeline (informers, `WatchCRDs`, `Run`) without needing a NATS
  broker. `ListenToRequests` is the only broker-only goroutine, and you rarely
  need it. The written snapshot doubles as proof discovery worked end-to-end.
- **`DEBUG=true` is required** to see the discovery/resync debug lines (see
  `main.go`: it maps `DEBUG=true|1` to logrus debug level).

## Drive the surface

Pick the smallest cluster action that makes the changed code run. Examples:

| Changed area | Drive it with |
|---|---|
| CRD watch / informer resync (`handleCRDEvent`, `updatePipelineConfig`) | `kubectl annotate crd <name> k=v$RANDOM --overwrite` (MODIFIED); `kubectl apply`/`delete` a dummy CRD (ADDED/DELETED) |
| Resource discovery / publishing | create/patch/delete a watched resource (`kubectl create deploy`, `kubectl label`, ...) |
| Namespace/resource filtering (`--outputNamespaces`, `--outputResources`) | launch with the flag, create resources in/out of scope, check they are/aren't in the snapshot |
| whitelist/blacklist config | edit the CR's `watch-list`, restart the binary, check which kinds appear |

For a MODIFIED burst use a *changing* annotation value each time (a repeated
identical value is a no-op and emits no watch event).

## Observe: signal lines

Grep the log for the lines that mark the behavior. These are current as of this
writing - **grep the source to confirm they still exist** before relying on them,
since log strings drift:

| Signal | Log substring | Source |
|---|---|---|
| A resync was requested for a CRD event | `Resyncing informer from watch crd` | `meshsync/handlers.go` `handleCRDEvent` |
| A CRD event was correctly ignored | `did not change discovery config; skipping informer resync` | `meshsync/handlers.go` `handleCRDEvent` |
| The informer factory was torn down/rebuilt | `Creating new dynamic shared informer factory` | `meshsync/handlers.go` `UpdateInformer` |
| A resource event was published | `Received ADD event for` / `UPDATE` / `DELETE` | `internal/pipeline/handlers.go` |
| Watch re-established / backoff | `crdWatcher.ResultChan() was closed`, `watch iteration failed`, `watch collapsed` | `meshsync/handlers.go` `WatchCRDs` |

Count them and reconcile against what you drove - a bare "it logged the right
line" is weaker than "I drove N events and got exactly the N signals I expected,
and the counts add up". Use a log mark so you only look at lines after an action:

```bash
LOG="$($SKILL logs)"; MARK=$(wc -l < "$LOG")
# ... drive events, then wait past the 5s resync debounce ...
tail -n +$((MARK+1)) "$LOG" | grep -E 'Resyncing informer|did not change discovery|dynamic shared informer factory' \
  | sed -E 's/.*msg="//; s/".*//' | sort | uniq -c
```

Also sanity-check the whole run: `grep -iE 'level=error|panic:' "$LOG"`.

## Reconcile and clean up

Reconcile the totals so the picture is airtight, e.g. "resync-triggers = startup
ADDs + my ADD + my DELETE; modified-skips = my MODIFIED burst + the trailing
status/finalizer MODIFIEDs k8s emits on create/delete". Then `$SKILL down` and
confirm the process is gone (`pgrep -f meshsync-bin`).

## Worked example: the CRD re-list storm fix

Verifying that CRD `MODIFIED` events no longer trigger a full informer resync
(the cert-manager cainjector `caBundle` storm):

1. `up` + `run`, wait ~10s. Startup lists existing CRDs as ADDED; genuinely new
   ones (e.g. `brokers`, `meshsyncs`) each drive one resync, coalesced by the 5s
   debounce into one `Creating new dynamic shared informer factory`.
2. Fire 5 MODIFIED events: `for i in 1 2 3 4 5; do kubectl annotate crd brokers.meshery.io storm=$i-$RANDOM --overwrite; sleep 1; done`, wait 8s.
   Expect **5x `did not change discovery config; skipping informer resync`, 0 resyncs, 0 rebuilds**.
3. `kubectl apply` a dummy CRD -> **1 resync + 1 rebuild**; its trailing status
   MODIFIEDs are **skipped**. `kubectl delete` it -> **1 resync + 1 rebuild**;
   finalizer MODIFIEDs skipped.
4. Confirm the snapshot has resources (`grep -c 'kind:' "$WORKDIR/snapshot.yaml"`)
   and the log has no errors.

Runtime-observable paths NOT covered by this harness: idempotent re-list on watch
re-establishment and exponential backoff both need a watch close / API-server
failure you can't force cheaply - lean on the unit tests for those and say so.

## Report

Follow the `/verify` report format: verdict, claim, method, numbered steps each
with what you did to the running agent and what it logged, an evidence sample
(the grepped signal lines), and findings. Never report PASS from tests or a clean
build - only from the agent actually doing the thing at its surface.
