#!/usr/bin/env bash
#
# verify-env.sh - stand up / launch / tear down a local MeshSync runtime for
# verification. This scripts only the boring, deterministic parts (cluster
# handle, CRD+CR bring-up, launch, cleanup) so the human/agent driving the
# verification can focus on the behavior actually under test.
#
# Subcommands:
#   up     Pin an isolated kubeconfig to a reachable LOCAL cluster and install
#          the meshsync CRD + CR (required for WatchCRDs to run).
#   run    Build the meshsync binary and launch it in file-output mode with
#          DEBUG logging, backgrounded, logs captured.
#   logs   Print the path to the running log (tail it yourself).
#   down   Stop meshsync and remove everything 'up' created.
#
# State (kubeconfig, pid, log) lives in $WORKDIR so subcommands share it.
#
# Env overrides:
#   WORKDIR                  scratch dir (default: ${TMPDIR:-/tmp}/meshsync-verify)
#   MESHSYNC_VERIFY_CONTEXT  force a specific kube context instead of autodetect
#   MESHERY_CRDS_URL         override the meshery-operator CRDs manifest URL

set -euo pipefail

WORKDIR="${WORKDIR:-${TMPDIR:-/tmp}/meshsync-verify}"
KC="$WORKDIR/kubeconfig.yaml"
PIDFILE="$WORKDIR/meshsync.pid"
LOG="$WORKDIR/meshsync.log"
SNAPSHOT="$WORKDIR/snapshot.yaml"
MESHERY_CRDS_URL="${MESHERY_CRDS_URL:-https://raw.githubusercontent.com/meshery/meshery/refs/heads/master/install/kubernetes/helm/meshery-operator/crds/crds.yaml}"

REPO_ROOT="$(git rev-parse --show-toplevel)"
mkdir -p "$WORKDIR"

# Candidate local contexts, in preference order. The DEFAULT kubeconfig context
# is deliberately NOT trusted - in practice it often points at an unreachable
# remote cluster, and meshsync would silently try to use it.
CANDIDATE_CONTEXTS=(docker-desktop orbstack rancher-desktop minikube kind-kind colima)

reachable() { kubectl --context="$1" get --raw='/readyz' --request-timeout=6s >/dev/null 2>&1; }

pick_context() {
  if [[ -n "${MESHSYNC_VERIFY_CONTEXT:-}" ]]; then
    reachable "$MESHSYNC_VERIFY_CONTEXT" || { echo "forced context '$MESHSYNC_VERIFY_CONTEXT' is not reachable" >&2; exit 1; }
    echo "$MESHSYNC_VERIFY_CONTEXT"; return
  fi
  # Try known local names, then any context whose name contains kind/minikube.
  local ctx
  for ctx in "${CANDIDATE_CONTEXTS[@]}"; do
    if kubectl config get-contexts -o name 2>/dev/null | grep -qx "$ctx" && reachable "$ctx"; then
      echo "$ctx"; return
    fi
  done
  while read -r ctx; do
    if reachable "$ctx"; then echo "$ctx"; return; fi
  done < <(kubectl config get-contexts -o name 2>/dev/null | grep -iE 'kind|minikube|desktop|orbstack|colima' || true)
  echo "" ; return
}

cmd_up() {
  local ctx; ctx="$(pick_context)"
  if [[ -z "$ctx" ]]; then
    echo "No reachable local cluster found." >&2
    echo "Start one (e.g. Docker Desktop Kubernetes, minikube start), or run" >&2
    echo "  $REPO_ROOT/integration-tests/infrastructure/setup.sh setup" >&2
    echo "to create a dedicated kind cluster, then re-run: $0 up" >&2
    exit 1
  fi
  echo "Using local context: $ctx"
  kubectl config view --minify --context="$ctx" --raw > "$KC"
  export KUBECONFIG="$KC"
  echo "Pinned isolated kubeconfig -> $KC"

  kubectl get ns meshery >/dev/null 2>&1 || kubectl create namespace meshery
  # The MeshSync CRD (meshsyncs.meshery.io) gates useCRDFlag, which gates
  # WatchCRDs. Without the CRD *and* the meshery-meshsync CR, WatchCRDs never
  # starts and CRD-event handling can't be observed at all.
  kubectl apply -f "$MESHERY_CRDS_URL"
  kubectl apply -f "$REPO_ROOT/integration-tests/infrastructure/meshsync.yaml"
  kubectl -n meshery get meshsync meshery-meshsync
  echo "Cluster ready. Export KUBECONFIG=$KC for kubectl commands against it."
}

cmd_run() {
  [[ -f "$KC" ]] || { echo "run '$0 up' first (no kubeconfig at $KC)" >&2; exit 1; }
  echo "Building meshsync binary..."
  ( cd "$REPO_ROOT" && go build -o "$WORKDIR/meshsync-bin" . )
  : > "$LOG"
  DEBUG=true KUBECONFIG="$KC" "$WORKDIR/meshsync-bin" \
      --output file --outputFile "$SNAPSHOT" > "$LOG" 2>&1 &
  echo "$!" > "$PIDFILE"
  echo "meshsync PID $(cat "$PIDFILE"), log: $LOG, snapshot: $SNAPSHOT"
  echo "Give it ~10s to connect and finish initial discovery before driving events."
}

cmd_logs() { echo "$LOG"; }

cmd_down() {
  [[ -f "$PIDFILE" ]] && kill "$(cat "$PIDFILE")" 2>/dev/null && echo "meshsync stopped" || true
  rm -f "$PIDFILE"
  if [[ -f "$KC" ]]; then
    export KUBECONFIG="$KC"
    kubectl delete -f "$REPO_ROOT/integration-tests/infrastructure/meshsync.yaml" >/dev/null 2>&1 || true
    kubectl delete crd brokers.meshery.io meshsyncs.meshery.io >/dev/null 2>&1 || true
    kubectl delete namespace meshery >/dev/null 2>&1 || true
    echo "removed meshery CRDs, CR, and namespace"
  fi
}

case "${1:-}" in
  up)   cmd_up ;;
  run)  cmd_run ;;
  logs) cmd_logs ;;
  down) cmd_down ;;
  *)    echo "usage: $0 {up|run|logs|down}" >&2; exit 2 ;;
esac
