# kubepatch

**Patch-based, environment-aware Kubernetes deployments using plain YAML and zero templating**

---

## 🎯 Why kubepatch?

Unlike tools that embed logic into YAML or require custom template languages, `kubepatch` keeps
your **base manifests clean and idiomatic**.

* **Simple**: No templates, DSLs, or logic in YAML, zero magic
* **Predictable**: No string substitutions or regex hacks
* **Safe**: Only native Kubernetes YAML manifests - readable, valid, untouched
* **Layered**: Patch logic is externalized and explicit via JSON Patch (RFC 6902)
* **Declarative**: Cross-environment deployment with predictable, understandable changes

---

## 🛠 Example

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

## ✨ Key Features

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

### Env Var Substitution (in Patch Values Only)

You can inject secrets and configuration values directly into patch files:

```yaml
- op: add
  path: /spec/template/spec/containers/0/env
  value:
    - name: PGPASSWORD
      value: ${IAM_SERVICE_PGPASS}
```

Strict env-var substitution (prefix-based) is only allowed inside patches - never in base manifests.

---

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
