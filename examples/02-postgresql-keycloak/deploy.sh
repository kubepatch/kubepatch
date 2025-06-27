#!/bin/bash
set -euo pipefail

export KC_ADMIN_USER=admin
export KC_ADMIN_PASS=admin
export DB_USER=postgres
export DB_PASSWORD=postgres

kubepatch patch -f k8s/manifests/ -p k8s/patches.yaml --envsubst-prefixes='DB_,KC_'
