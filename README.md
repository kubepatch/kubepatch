# kubepatch

**Patch-based, environment-aware Kubernetes deployments using plain YAML and zero templating**

[![License](https://img.shields.io/github/license/kubepatch/kubepatch)](https://github.com/kubepatch/kubepatch/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubepatch/kubepatch)](https://goreportcard.com/report/github.com/kubepatch/kubepatch)
[![Go Reference](https://pkg.go.dev/badge/github.com/kubepatch/kubepatch.svg)](https://pkg.go.dev/github.com/kubepatch/kubepatch)
[![Workflow Status](https://img.shields.io/github/actions/workflow/status/kubepatch/kubepatch/ci.yml?branch=master)](https://github.com/kubepatch/kubepatch/actions/workflows/ci.yml?query=branch:master)
[![GitHub Issues](https://img.shields.io/github/issues/kubepatch/kubepatch)](https://github.com/kubepatch/kubepatch/issues)
[![Go Version](https://img.shields.io/github/go-mod/go-version/kubepatch/kubepatch)](https://github.com/kubepatch/kubepatch/blob/master/go.mod#L3)
[![Latest Release](https://img.shields.io/github/v/release/kubepatch/kubepatch)](https://github.com/kubepatch/kubepatch/releases/latest)
[![Start contributing](https://img.shields.io/github/issues/kubepatch/kubepatch/good%20first%20issue?color=7057ff&label=Contribute)](https://github.com/kubepatch/kubepatch/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22good+first+issue%22)

---

## Table of Contents

- [About](#about)
- [Example](#example)
- [Usage](#usage)
- [Installation](#installation)
    - [Binaries](#manual-installation)
    - [Packages](#package-based-installation)
    - [Docker Images](#docker-images)
- [Architecture](#architecture)
    - [JSON Patch Only](#json-patch-only)
    - [Plain Kubernetes YAML Manifests](#plain-kubernetes-yaml-manifests)
    - [Cross-Environment Deploys](#cross-environment-deploys)
    - [Common Labels Support](#common-labels-support)
    - [Env Var Substitution](#env-var-substitution)
- [Contributing](#contributing)
- [License](#license)

---

## About

Unlike tools that embed logic into YAML or require custom template languages, `kubepatch` keeps
your **base manifests clean and idiomatic**.

* **Simple**: No templates, DSLs, or logic in YAML, zero magic
* **Predictable**: No string substitutions or regex hacks
* **Safe**: Only native Kubernetes YAML manifests - readable, valid, untouched
* **Layered**: Patch logic is externalized and explicit via JSON Patch (RFC 6902)
* **Declarative**: Cross-environment deployment with predictable, understandable changes

---

## Example

Given a base set of manifests for deploy a basic microservice [see examples](examples/01-basic-microservice):

```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: myapp
  labels:
    app: myapp
spec:
  type: NodePort
  selector:
    app: myapp
  ports:
    - protocol: TCP
      port: 8080
      targetPort: 8080

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  labels:
    app: myapp
spec:
  replicas: 2
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
        - name: myapp
          image: "localhost:5000/restapiapp:latest"
```

A `patches/prod.yaml` might look like:

```yaml
name: myapp-prod

labels:
  app: myapp-prod

patches:
  # deployment
  - target:
      kind: Deployment
      name: myapp
    patches:
      - op: replace
        path: /spec/replicas
        value: 1

      - op: replace
        path: /spec/template/spec/containers/0/image
        value: "localhost:5000/restapiapp:1.21"

      - op: add
        path: /spec/template/spec/containers/0/env
        value:
          - name: RESTAPIAPP_VERSION
            value: prod
          - name: LOG_LEVEL
            value: info

      - op: add
        path: /spec/template/spec/containers/0/resources
        value:
          limits:
            cpu: "500m"
            memory: "512Mi"
          requests:
            cpu: "64m"
            memory: "128Mi"

  # service
  - target:
      kind: Service
      name: myapp
    patches:
      - op: add
        path: /spec/ports/0/nodePort
        value: 30266
```

A `patches/dev.yaml` might look like:

```yaml
name: myapp-dev

labels:
  app: myapp-dev

patches:
  # deployment
  - target:
      kind: Deployment
      name: myapp
    patches:
      - op: add
        path: /spec/template/spec/containers/0/env
        value:
          - name: RESTAPIAPP_VERSION
            value: dev
          - name: LOG_LEVEL
            value: debug

  # service
  - target:
      kind: Service
      name: myapp
    patches:
      - op: add
        path: /spec/ports/0/nodePort
        value: 30265
```

Apply the appropriate patch set based on the target environment.

```bash
kubepatch patch -f base/ -p patches/dev.yaml | kubectl apply -f -
```

Rendered manifest may look like this (note that all labels are set, as well as all patches are applied)

```yaml
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: myapp-dev
  name: myapp-dev
spec:
  ports:
    - nodePort: 30265
      port: 8080
      protocol: TCP
      targetPort: 8080
  selector:
    app: myapp-dev
  type: NodePort
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: myapp-dev
  name: myapp-dev
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp-dev
  template:
    metadata:
      labels:
        app: myapp-dev
    spec:
      containers:
        - env:
            - name: RESTAPIAPP_VERSION
              value: dev
            - name: LOG_LEVEL
              value: debug
          image: localhost:5000/restapiapp:1.22
          name: myapp
```

---

## Usage

**[`^        back to top        ^`](#table-of-contents)**

```bash
# Patch and apply all manifests in a directory
kubepatch patch -f manifests/ -p patches.yaml | kubectl apply -f -

# Read base manifests from stdin
cat manifests.yaml | kubepatch patch -f - -p patches.yaml | kubectl apply -f -

# Patch manifests recursively from nested directories
kubepatch patch -R -f manifests/ -p patches.yaml | kubectl apply -f -

# Patch and apply a manifest from a remote URL
kubepatch patch \
  -f https://raw.githubusercontent.com/user/repo/refs/heads/master/manifests/deployment.yaml \
  -p patches.yaml \
  | kubectl apply -f -

# Patch with environment variable substitution in patch-file (only variables prefixed with CI_ or APP_)
kubepatch patch -f manifests/ -p patches.yaml --envsubst-prefixes='CI_,APP_' | kubectl apply -f -
```

---

## Installation

**[`^        back to top        ^`](#table-of-contents)**

### Manual Installation

1. Download the latest binary for your platform from
   the [Releases page](https://github.com/kubepatch/kubepatch/releases).
2. Place the binary in your system's `PATH` (e.g., `/usr/local/bin`).

### Installation script

```bash
(
set -euo pipefail

OS="$(uname | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')"
TAG="$(curl -s https://api.github.com/repos/kubepatch/kubepatch/releases/latest | jq -r .tag_name)"

curl -L "https://github.com/kubepatch/kubepatch/releases/download/${TAG}/kubepatch_${TAG}_${OS}_${ARCH}.tar.gz" |
tar -xzf - -C /usr/local/bin && \
chmod +x /usr/local/bin/kubepatch
)
```

### Package-Based installation

#### Debian

```bash
sudo apt update -y && sudo apt install -y curl
curl -LO https://github.com/kubepatch/kubepatch/releases/latest/download/kubepatch_linux_amd64.deb
sudo dpkg -i kubepatch_linux_amd64.deb
```

#### Alpine Linux

```bash
apk update && apk add --no-cache bash curl
curl -LO https://github.com/kubepatch/kubepatch/releases/latest/download/kubepatch_linux_amd64.apk
apk add kubepatch_linux_amd64.apk --allow-untrusted
```

#### Docker Images

Pre-build images are available at [quay.io/kubepatch/kubepatch](https://quay.io/repository/kubepatch/kubepatch)

```bash
docker pull quay.io/kubepatch/kubepatch:latest
```

---

## Architecture

**[`^        back to top        ^`](#table-of-contents)**

### JSON Patch Only

Patches are applied using [JSON Patch](https://tools.ietf.org/html/rfc6902):

```yaml
- op: replace
  path: /spec/replicas
  value: 1
```

Every patch is minimal, explicit, and easy to understand. No string manipulation or text templating involved.

### Plain Kubernetes YAML Manifests

Your base manifests are 100% pure Kubernetes objects - no logic, no annotations, no overrides, no preprocessing. This
ensures:

* Easy editing
* Compatibility with other tools
* Clean Git diffs

### Cross-Environment Deploys

Deploy to `dev`, `staging`, or `prod` just by selecting the right set of patches. All logic lives in patch files, not
your base manifests.

### Common Labels Support

Inject common labels (like `env`, `team`, `app`), including deep paths like pod templates and selectors.

### Env Var Substitution

You can inject secrets and configuration values directly into patch files:

```yaml
- op: add
  path: /spec/template/spec/containers/0/env
  value:
    - name: PGPASSWORD
      value: ${IAM_SERVICE_PGPASS}
```

Strict env-var substitution (prefix-based) is only allowed inside patches - never in base manifests.

Fails if any placeholders remain unexpanded (unset envs, etc...), ensuring deployment predictability.

```
kubepatch patch -f base/ -p patches/dev.yaml --envsubst-prefixes='CI_,APP_,IMAGE_'
```

## Contributing

**[`^        back to top        ^`](#table-of-contents)**

Contributions are welcomed and greatly appreciated. See [CONTRIBUTING.md](./CONTRIBUTING.md)
for details on submitting patches and the contribution workflow.

---

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
