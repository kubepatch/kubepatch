#!/bin/bash
set -euo pipefail

echo "✓ Stopping and removing registry container..."
docker rm -f kind-registry 2>/dev/null || echo "Registry container not running"

echo "✓ Disconnecting registry from 'kind' network..."
docker network disconnect kind kind-registry 2>/dev/null || echo "Already disconnected"

echo "✓ Deleting kind cluster..."
kind delete cluster --name kubepatch || echo "Kind cluster not found"

echo "✓ Running local registry..."
docker run -d --restart=always -p 5000:5000 --name kind-registry registry:2

echo "✓ Running kind cluster..."
kind create cluster --config=kind-config.yaml
kubectl config set-context "kind-kubepatch"
docker network connect kind kind-registry
