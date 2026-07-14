# Cluster Autoscaler E2E Tests

End-to-end tests for **Cluster Autoscaler** behavior in the `node-manager` module, using [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/).

## Overview

These tests validate scale-from-zero, Priority Expander selection and fallback, safe-to-evict scale-down, and node label selector matching on DVP (Cluster API) and Yandex Cloud (MCM) clusters.

The root `Taskfile.yaml` includes each scenario as a separate Task include. Shared workload manifests live in `tests/common/manifests/deployment.yaml`: three `pause` replicas with `nodeSelector: app=e2e-autoscaler-test`, strict pod anti-affinity per node, and tolerations for `dedicated=worker-100` and `dedicated=worker-50` taints so pods land only on the intended test NodeGroups.

Each scenario lives in `tests/<name>/` and is executed via Task wrappers that call `chainsaw test` with JUnit reports in `./reports/`.

## Chainsaw

[Chainsaw](https://kyverno.github.io/chainsaw/) is a declarative e2e testing tool for Kubernetes. Tests are defined in YAML as a sequence of steps with `try`/`catch`/`finally` blocks. Chainsaw creates a temporary namespace per test, applies resources, runs assertions and scripts, and cleans up automatically.

**Key concepts:**

- `try` — main operations; the step fails if any operation fails
- `catch` — runs only on failure (diagnostics collection)
- `cleanup` — runs after the step completes (resource deletion)
- `$NAMESPACE` — auto-generated test namespace; the `e2e-nginx` Deployment is applied there

Shared Chainsaw settings are in `chainsaw-config.yaml` at the suite root (`failFast: true`, `parallel: 1`, test discovery via `chainsaw-test.yaml`).

## Prerequisites

### Tools

| Tool                                            | Purpose                                                  |
| ----------------------------------------------- | -------------------------------------------------------- |
| `kubectl`                                       | Cluster access; context must be set before running tests |
| [Chainsaw](https://kyverno.github.io/chainsaw/) | `chainsaw test`, `chainsaw lint`                         |
| [Task](https://taskfile.dev/)                   | Wrapper commands in `Taskfile.yml`                       |
| `jq`                                            | Clone `*InstanceClass` resources in test scripts         |

**Install Chainsaw**

Homebrew (macOS/Linux):

```bash
brew tap kyverno/chainsaw https://github.com/kyverno/chainsaw
brew install kyverno/chainsaw/chainsaw
```

Go install:

```bash
go install github.com/kyverno/chainsaw@latest
```

Or download a binary from [Chainsaw releases](https://github.com/kyverno/chainsaw/releases).

**Verify:**

```bash
chainsaw version
task --version
kubectl cluster-info
```

### Cluster requirements

**General (all scenarios):**

1. Deckhouse with the `node-manager` module and cloud nodes (`CloudEphemeral` NodeGroups).
2. Cluster Autoscaler deployed and Ready:
   - Deployment `cluster-autoscaler` in namespace `d8-cloud-instance-manager`
   - CA is enabled when at least one `NodeGroup` has `nodeType: CloudEphemeral` and `minPerZone < maxPerZone` (see `cluster_autoscaler_enabled` in `templates/_helpers.yaml`)
3. Priority Expander configured: `--expander=priority,least-waste` in CA args (module default).
4. NodeGroup priorities published to ConfigMap `cluster-autoscaler-priority-expander` via the `set_ng_priorities` hook from `spec.cloudInstances.priority`.
5. RBAC to create/delete `NodeGroup`, `DVPInstanceClass` / `YandexInstanceClass`, and `Deployment`; restart CA; read logs and events.

**DVP scenarios (`*-dvp`):**

- DVP provider (or another provider with `cloud-provider=clusterapi` in CA args)
- Existing `DVPInstanceClass` named `worker` (template for cloning)
- CA container args contain `clusterapi`

**Yandex scenarios (`*-yandex`):**

- Yandex Cloud with MCM
- Existing `YandexInstanceClass` named `worker`
- CA container args contain `mcm`

**Resources and cost:**

- Tests create NodeGroups `e2e-worker-100`, `e2e-worker-50`, and/or other `e2e-*` groups, clone instance classes, and deploy `e2e-nginx` (3 replicas with anti-affinity → up to 3 new VMs).
- Tests restart `cluster-autoscaler`.
- Scale-from-zero scenarios typically take tens of minutes; fallback scenarios up to 30–60 minutes (Yandex longer).
- Cleanup removes test resources; nodes labeled `app=e2e-autoscaler-test` are polled for up to ~10 minutes (timeout logs a warning, does not fail the test).

## Directory Structure

```text
cluster-autoscaler/
├── Taskfile.yaml              # includes all scenarios
├── chainsaw-config.yaml       # shared timeouts and execution settings
└── tests/
    ├── common/manifests/      # shared e2e-nginx Deployment
    ├── ca-scale-from-zero-*/
    ├── ca-priority-fallback-*/
    ├── ca-safe-to-evict-*/
    └── ca-scale-from-zero-node-label-dvp/
```

Per-scenario details (steps, manifests, expected outcomes): `tests/<name>/<name>.md`.

## Available Tests

| Task command                                 | Provider   | Description                                                            | Expected node group                                       |
| -------------------------------------------- | ---------- | ---------------------------------------------------------------------- | --------------------------------------------------------- |
| `task ca-scale-from-zero-dvp:run`            | clusterapi | Priority Expander selects high-priority group on scale-from-zero       | All pods on `e2e-worker-100`; no nodes in `e2e-worker-50` |
| `task ca-scale-from-zero-yandex:run`         | mcm        | Same as DVP for Yandex Cloud                                           | All pods on `e2e-worker-100`; no nodes in `e2e-worker-50` |
| `task ca-priority-fallback-dvp:run`          | clusterapi | Fallback to lower-priority group when top-priority group is broken     | All pods on `e2e-worker-50`                               |
| `task ca-priority-fallback-yandex:run`       | mcm        | Same fallback with invalid Yandex image                                | All pods on `e2e-worker-50`                               |
| `task ca-safe-to-evict-dvp:run`              | clusterapi | Scale-down with `safe-to-evict: "true"` annotation on a standalone pod | Node removed after Deployment deleted                     |
| `task ca-safe-to-evict-yandex:run`           | mcm        | Same safe-to-evict scale-down for Yandex                               | Node removed after Deployment deleted                     |
| `task ca-scale-from-zero-node-label-dvp:run` | clusterapi | Scale-from-zero with `nodeSelector` on `node.deckhouse.io/group`       | CA matches system labels in capacity annotation           |

**DVP vs Yandex differences (scale-from-zero / fallback):**

- DVP: create instance class → restart CA; read logs from `cluster-autoscaler` container only
- Yandex: restart CA before creating cloned instance class; read logs with `--all-containers --since=10m` (fallback: `--since=90m`, up to 60 min wait)

## Running Tests

### Validate without a cluster

From a scenario directory:

```bash
cd modules/040-node-manager/e2e/cluster-autoscaler/tests/ca-scale-from-zero-dvp
task dry-run
```

From the suite root (all scenarios):

```bash
cd modules/040-node-manager/e2e/cluster-autoscaler
task dry-run
```

### Run a single scenario

From a scenario directory:

```bash
cd modules/040-node-manager/e2e/cluster-autoscaler/tests/ca-scale-from-zero-yandex

task run          # full output + JUnit in ./reports/
task run:quiet    # errors and summary only
task run:debug    # pause on failure + fail-fast
```

From the suite root via includes:

```bash
cd modules/040-node-manager/e2e/cluster-autoscaler

task ca-scale-from-zero-dvp:run
task ca-scale-from-zero-yandex:run
task ca-priority-fallback-dvp:run
task ca-priority-fallback-yandex:run
task ca-safe-to-evict-dvp:run
task ca-safe-to-evict-yandex:run
task ca-scale-from-zero-node-label-dvp:run
```

### Run all scenarios

```bash
cd modules/040-node-manager/e2e/cluster-autoscaler
task run
```

Run only scenarios matching your cloud provider (DVP or Yandex).

### Direct Chainsaw invocation

```bash
cd modules/040-node-manager/e2e/cluster-autoscaler/tests/ca-scale-from-zero-dvp

mkdir -p reports
chainsaw test --test-dir . \
  --config ../../chainsaw-config.yaml \
  --parallel 1 \
  --report-format JUNIT-STEP \
  --report-path ./reports/
```

### kubectl context

Chainsaw uses the current `kubectl` context. Before running:

```bash
kubectl config current-context
kubectl get deployment cluster-autoscaler -n d8-cloud-instance-manager
kubectl get dvpinstanceclass worker      # DVP scenarios
# or
kubectl get yandexinstanceclass worker   # Yandex scenarios
```

## Timeouts

From `chainsaw-config.yaml`:

| Timeout | Value |
| ------- | ----- |
| apply   | 60s   |
| assert  | 15m   |
| error   | 15m   |
| delete  | 5m    |
| cleanup | 20m   |
| exec    | 5m    |

Individual steps poll CA logs for 5–30 minutes; Yandex fallback scenarios allow up to 60 minutes. Plan 30–90 minutes per full run depending on scenario and cloud provider.

## Reports and Debugging

- JUnit reports: `tests/<scenario>/reports/chainsaw-report.xml` (`reports/` is gitignored)
- On failure, tests collect events and CA pod logs from `d8-cloud-instance-manager`
- Useful manual checks:

```bash
kubectl logs -n d8-cloud-instance-manager -l app=cluster-autoscaler -c cluster-autoscaler --tail=200
kubectl get nodegroup e2e-worker-100 e2e-worker-50
kubectl get nodes -l app=e2e-autoscaler-test
```

## Troubleshooting

| Symptom                                | Likely cause                                                 |
| -------------------------------------- | ------------------------------------------------------------ |
| CA deployment missing                  | No `CloudEphemeral` NodeGroup with `minPerZone < maxPerZone` |
| Assert on `worker` InstanceClass fails | Base `worker` NodeGroup or instance class does not exist     |
| `grep clusterapi` / `grep mcm` fails   | Wrong scenario for your cloud provider                       |
| Timeout waiting for priority logs      | Slow cloud, quotas, or MCM/CAPI errors                       |
| Fallback timeout                       | Long backoff; Yandex scenarios allow up to 1 hour            |
