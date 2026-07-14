---
title: Nelm annotations
permalink: en/architecture/marketplace/nelm-annotations.html
description: "Nelm annotations for Application deployment: ordering, lifecycle, tracking, logging, template functions, deployment stages, and common use cases."
---

{% raw %}
Application templates are rendered and deployed by **Nelm**. Nelm extends the standard Helm behavior with annotations that control deployment order, resource lifecycle, readiness tracking, and log output. This page covers all annotations available in Application templates.

## Deployment stages

Nelm processes a deployment in three stages. Different annotation groups affect different stages.

### 1. Render

Templates are evaluated with the current values. No cluster access is needed (except for `lookup`). At this stage:

- `werf.io/deploy-on` controls whether a resource is included in the render output for the current operation type (`install`, `upgrade`, etc.).
- Template functions (`werf_secret_file`, `dump_debug`, etc.) are executed.
- `secret-values.yaml` and files from `secret/` are decrypted.

### 2. Plan

Nelm connects to the cluster, reads current resource state, and runs a **dry-run Server-Side Apply** to compute the exact diff. It then builds a **DAG of operations** — which resources to create, update, or delete, and in what order.

At this stage:
- Order and dependency annotations (`werf.io/weight`, `werf.io/deploy-dependency-*`, `*.external-dependency.werf.io/*`) shape the DAG.
- Lifecycle annotations (`werf.io/ownership`, `werf.io/delete-policy`, `werf.io/delete-propagation`) determine which operations enter the plan.

Dry-run SSA means the diff is computed by the API server — defaulting, admission plugins, and mutating webhooks are all applied.

### 3. Apply

The DAG is executed: resources are created, updated, or deleted in dependency order, with parallelism where no dependencies exist. Readiness tracking and log streaming run concurrently.

At this stage:
- Tracking annotations (`werf.io/track-termination-mode`, `werf.io/fail-mode`, `werf.io/failures-allowed-per-replica`, `werf.io/no-activity-timeout`) control waiting behavior.
- Log annotations control what is printed during deployment.

---

## Common use cases

### 1. Job that must run before the main application

Classic case: database migration before the application starts.

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{.Application.Instance.Name}}-db-migrate
  annotations:
    werf.io/delete-policy: before-creation
spec: ...
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Application.Instance.Name}}-app
  annotations:
    werf.io/deploy-dependency-migrate: state=ready,kind=Job,name=db-migrate
spec: ...
```

`before-creation` ensures the Job is recreated on each deploy (Kubernetes otherwise rejects updates to immutable Job fields). The Deployment waits for `ready` — i.e., successful job completion.

### 2. Preserve a resource across uninstall or chart removal

Use case: a PVC with database data that must not be deleted even when the Application is uninstalled.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{.Application.Instance.Name}}-postgres-data
  annotations:
    helm.sh/resource-policy: keep
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 50Gi
  storageClassName: gp3
```

`helm.sh/resource-policy: keep` prevents the PVC from being deleted on uninstall or when removed from the chart. It does not protect against `d8 k delete pvc` — for that, use `persistentVolumeReclaimPolicy: Retain` on the StorageClass.

### 3. Resource shared between releases

A TLS Secret used by multiple charts should not disappear when any one of them is uninstalled.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{.Application.Instance.Name}}-shared-tls
  annotations:
    werf.io/ownership: anyone
type: kubernetes.io/tls
data: ...
```

`anyone` tells Nelm it is not the sole owner, so it skips deletion on uninstall.

### 4. Wait for a resource created by an operator

Deploy only after cert-manager issues a certificate:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Application.Instance.Name}}-app
  annotations:
    cert.external-dependency.werf.io/resource: certificates.v1.cert-manager.io/myapp-tls
spec: ...
```

Nelm waits for the `Certificate` to be `present` and `ready` before creating the Deployment.

### 5. Non-critical component that must not fail the deploy

A metrics DaemonSet whose unavailability should not block the release:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: {{.Application.Instance.Name}}-metrics-agent
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
spec: ...
```

Nelm will not wait for readiness and will not fail on timeout.

### 6. Resource rendered only on first install

An init Job that runs only during `install`, not `upgrade`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{.Application.Instance.Name}}-init-data
  annotations:
    werf.io/deploy-on: install
    werf.io/ownership: anyone   # CRITICAL
spec: ...
```

Without `werf.io/ownership: anyone`, on upgrade the resource renders as absent and the owning release removes it. `anyone` prevents that.

### 7. Slow-starting resource with a large image

StatefulSet with a large Docker image (ML models, Elasticsearch with pre-loaded indices):

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: elasticsearch
  annotations:
    werf.io/no-activity-timeout: 20m
    werf.io/failures-allowed-per-replica: "3"
    werf.io/show-service-messages: "true"
spec: ...
```

- `no-activity-timeout: 20m` — 20 minutes without events before Nelm considers it timed out. Use this for slow image pulls or long initialization.
- `failures-allowed-per-replica: "3"` — allows up to 3 restarts per replica before declaring failure. Use for flapping init dependencies.
- `show-service-messages: "true"` — emit Kubernetes Events in deploy output (useful for diagnosing `ImagePullBackOff`, scheduling failures, OOM events).

---

## Ordering and dependencies

### Weights

`werf.io/weight` groups resources: equal weights deploy in parallel; different weights deploy sequentially in ascending order.

```yaml
metadata:
  annotations:
    werf.io/weight: "-10"   # Deploys before default-weight (0) resources
```

### Direct dependencies

`werf.io/deploy-dependency-<id>` waits for a specific resource in the same release to reach a specific state before deploying the annotated resource.

```yaml
metadata:
  annotations:
    werf.io/deploy-dependency-db: state=ready,kind=StatefulSet,name=postgres
    werf.io/deploy-dependency-migrations: state=present,kind=Job,name=db-migrate
```

Dependency states:

- `ready` — resource is in a ready state (e.g., Deployment's `availableReplicas == replicas`).
- `present` — resource exists in the cluster.

Full format:

```text
werf.io/deploy-dependency-<id>: state=ready|present[,name=<name>][,namespace=<namespace>][,kind=<kind>][,group=<group>][,version=<version>]
```

{% endraw %}
{% alert level="warning" %}
This annotation has no effect if the dependency resource is in a different deploy stage (pre/main/post). Cross-stage ordering is already enforced by the stage sequence itself.
{% endalert %}
{% raw %}

### External dependencies

`<id>.external-dependency.werf.io/resource` waits for a resource **outside the release** (created by an operator or another release):

```yaml
metadata:
  annotations:
    cert.external-dependency.werf.io/resource: certificates.v1.cert-manager.io/myapp-tls
    cert.external-dependency.werf.io/name: myapp-production   # Namespace of the external resource
```

Full format:

```text
<id>.external-dependency.werf.io/resource: <kind>[.<version>.<group>]/<name>
```

### Delete dependencies

`werf.io/delete-dependency-<id>` — the annotated resource will be deleted only after the referenced resource becomes `absent`:

```yaml
metadata:
  annotations:
    werf.io/delete-dependency-app: state=absent,kind=Deployment,name=app
```

---

## Lifecycle annotations

### `helm.sh/resource-policy`

`keep` — do not delete the resource on uninstall or when removed from the chart. The resource continues to be updated on install/upgrade as long as it renders.

### `werf.io/ownership`

- `release` (default for regular resources) — resource is deleted on uninstall and when absent from the chart. Release metadata annotations are applied.
- `anyone` (default for hooks and CRDs in `crds/`) — resource is not deleted on uninstall. Release metadata annotations are not applied.

Use `anyone` for resources shared between releases or for resources that should survive their release (init Jobs with `werf.io/deploy-on: install`).

### `werf.io/deploy-on`

Controls on which lifecycle operations the resource is rendered.

```yaml
werf.io/deploy-on: pre-install,upgrade,post-install
```

Allowed values: `pre-install`, `install`, `post-install`, `pre-upgrade`, `upgrade`, `post-upgrade`, `pre-rollback`, `rollback`, `post-rollback`, `pre-uninstall`, `uninstall`, `post-uninstall`. Default: `install,upgrade,rollback`.

{% endraw %}
{% alert level="warning" %}
If a resource is rendered for `install` but not `upgrade`, and `werf.io/ownership: release` is set, the resource will be **deleted on upgrade** because it is absent from the upgrade render. Set `werf.io/ownership: anyone` to prevent this.
{% endalert %}
{% raw %}

### `werf.io/delete-policy`

Controls when a resource is deleted relative to the apply operation.

| Value | When |
|---|---|
| `before-creation` | Always recreate before apply |
| `before-creation-if-immutable` | Recreate only on "field is immutable" error (default for Jobs) |
| `succeeded` | Delete after successful deploy |
| `failed` | Delete on readiness-check failure |

Values can be combined with a comma: `before-creation,succeeded`.

### `werf.io/delete-propagation`

Kubernetes deletion propagation strategy.

| Value | Behavior |
|---|---|
| `Foreground` (default) | Wait for dependent objects to be deleted |
| `Background` | Delete resource immediately; dependents deleted asynchronously |
| `Orphan` | Delete resource; leave dependents |

---

## Tracking annotations

| Annotation | Default | Description |
|---|---|---|
| `werf.io/track-termination-mode` | `WaitUntilResourceReady` | `WaitUntilResourceReady` or `NonBlocking` |
| `werf.io/fail-mode` | `FailWholeDeployProcessImmediately` | `FailWholeDeployProcessImmediately` or `IgnoreAndContinueDeployProcess` |
| `werf.io/failures-allowed-per-replica` | `1` | Number of restarts per replica before declaring failure |
| `werf.io/no-activity-timeout` | `4m` | Go duration; timeout on absence of events or status changes |
| `werf.io/show-service-messages` | `false` | Show Kubernetes Events in deploy output |

---

## Log annotations

| Annotation | Default | Description |
|---|---|---|
| `werf.io/skip-logs` | `false` | Suppress all pod logs |
| `werf.io/skip-logs-for-containers` | — | Comma-separated list of containers to suppress |
| `werf.io/show-logs-only-for-containers` | — | Comma-separated list; only show logs for these containers |
| `werf.io/show-logs-only-for-number-of-replicas` | `1` | Show logs only for the first N replicas |
| `werf.io/log-regex` | — | RE2 pattern; show only matching lines |
| `werf.io/log-regex-skip` | — | RE2 pattern; suppress matching lines |
| `werf.io/log-regex-for-<container>` | — | Per-container RE2 filter for showing |
| `werf.io/log-regex-skip-for-<container>` | — | Per-container RE2 filter for suppressing |

---

## Template functions

### `werf_secret_file`

Embed the decrypted content of a file from the `secret/` directory:

```yaml
data:
  config.yaml: {{ werf_secret_file "config.yaml" | b64enc }}
```

Files in `secret/` are stored encrypted (AES-128-CBC, key from `NELM_SECRET_KEY`). They are decrypted in memory during render. Use for certificates, private keys, and large configs that don't fit in `secret-values.yaml`.

### `dump_debug`, `printf_debug`, `include_debug`, `tpl_debug`

Debug-level output functions that emit to logs without affecting rendered output. Activated only when debug log level is set by the deploy system.

```yaml
{{ dump_debug $ }}
{{ printf_debug "replicaCount: %d" .Values.replicaCount }}
{{ include_debug "myapp.labels" . | nindent 4 }}
{{ tpl_debug "{{ .Values.template }}" . }}
```

---

## Known pitfalls

1. **`null` values and SSA.** Server-Side Apply frequently fails on fields with a `null` value. If `.Values.foo` is `nil`, the manifest gets `foo: null`. Fix with `{{ if .Values.foo }}` guards or `default`.

2. **`lookup` timing.** If cluster resources change between the Plan and Apply stages, the rendered plan may be stale. Avoid `lookup` for critical logic; pass data through values instead.

3. **Non-deterministic functions.** `now`, `randAlphaNum`, and map iteration with variable key ordering produce different renders on each call. With plan freezing this can create spurious diffs.

4. **`werf.io/deploy-dependency-*` and stages.** The annotation has no effect across deploy stages (pre/main/post). Cross-stage ordering is implicit in the stage sequence.
{% endraw %}
