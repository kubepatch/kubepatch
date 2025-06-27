#!/bin/bash
set -euo pipefail

export DB_USER=postgres
export DB_PASSWORD=postgres

kubepatch patch -f k8s/manifests/ -p k8s/patches.yaml --envsubst-prefixes='DB_'
