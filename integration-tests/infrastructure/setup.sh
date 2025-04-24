#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="meshsync-integration-test-cluster"
CUSTOM_NAMESPACE="agile-otter"


check_dependencies() {
  # Check for docker
  if ! command -v docker &> /dev/null; then
  echo "❌ docker is not installed. Please install docker first."
  exit 1
  fi
  echo "✅ docker is installed;"

  # Check for kind
  if ! command -v kind &> /dev/null; then
  echo "❌ kind is not installed. Please install KinD first."
  exit 1
  fi
  echo "✅ kind is installed;"

  # Check for kubectl
  if ! command -v kubectl &> /dev/null; then
  echo "❌ kubectl is not installed. Please install kubectl first."
  exit 1
  fi
  echo "✅ kubectl is installed;"
}

setup() {
  check_dependencies
  echo "🔧 Setting up..."

  echo "Running docker compose..."
  docker compose -f $SCRIPT_DIR/docker-compose.yaml up -d || exit 1

  echo "Creating KinD cluster..."
  kind create cluster --name $CLUSTER_NAME

  echo "Creating meshery namespace..."
  kubectl create namespace meshery

  echo "Applying meshery resources..."
  kubectl apply -f https://raw.githubusercontent.com/meshery/meshery/refs/heads/master/install/kubernetes/helm/meshery-operator/crds/crds.yaml
  kubectl --namespace meshery apply -f $SCRIPT_DIR/meshsync.yaml

  echo "Creating $CUSTOM_NAMESPACE namespace..."
  kubectl create namespace $CUSTOM_NAMESPACE

  echo "Applying $CUSTOM_NAMESPACE resources..."
  kubectl --namespace $CUSTOM_NAMESPACE apply -f $SCRIPT_DIR/test-deployment.yaml

  echo "Outputing cluster resources..."
  kubectl --namespace default get deployment
  kubectl --namespace default get rs
  kubectl --namespace default get po
  kubectl --namespace default get service
  kubectl --namespace default get configmap
  kubectl --namespace $CUSTOM_NAMESPACE get deployment
  kubectl --namespace $CUSTOM_NAMESPACE get rs
  kubectl --namespace $CUSTOM_NAMESPACE get po
  kubectl --namespace $CUSTOM_NAMESPACE get service
  kubectl --namespace $CUSTOM_NAMESPACE get configmap
}

cleanup() {
  echo "🧹 Cleaning up..."

  echo "Stopping docker compose..."
  docker compose -f $SCRIPT_DIR/docker-compose.yaml down

  echo "Deleting KinD cluster..."
  kind delete cluster --name $CLUSTER_NAME
}

print_help() {
  echo "Usage: $0 {check_dependencies|setup|cleanup|help}"
}

# Main dispatcher
case "$1" in
  check_dependencies)
    check_dependencies
    ;;
  setup)
    setup
    ;;
  cleanup)
    cleanup
    ;;
  help)
    print_help
    ;;
  *)
    echo "❌ Unknown command: $1"
    print_help
    exit 1
    ;;
esac