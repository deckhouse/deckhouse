# Descheduler E2E Tests

End-to-end tests for the `descheduler` module, using [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/).

## Overview

These tests validate Descheduler behavior in a Deckhouse cluster: pod rebalancing via LowNodeUtilization and HighNodeUtilization strategies, the Deckhouse patch that excludes `d8-*` and `kube-system` namespaces from eviction, and StatefulSet-specific behavior (RemoveDuplicates redistribution, PodDisruptionBudget handling, single-replica eviction, and the unsupported `minReplicas` knob).

Each scenario lives in `tests/<name>/` and is executed via Task wrappers that call `chainsaw test` with JUnit reports in `./reports/`.

## Chainsaw

[Chainsaw](https://kyverno.github.io/chainsaw/) is a declarative e2e testing tool for Kubernetes. Tests are defined in YAML as a sequence of steps with `try`/`catch`/`finally` blocks. Chainsaw creates a temporary namespace per test, applies resources, runs assertions and scripts, and cleans up automatically.

**Key concepts:**

- `try` — main operations; the step fails if any operation fails
- `catch` — runs only on failure (diagnostics collection)
- `cleanup` — runs after the step completes (resource deletion)
- `$NAMESPACE` — auto-generated test namespace, available in scripts

Shared Chainsaw settings are in `chainsaw-config.yaml` at the suite root (`failFast: true`, `parallel: 1`, test discovery via `chainsaw-test.yaml`).

## Prerequisites

### Tools

| Tool                                            | Purpose                                                  |
| ----------------------------------------------- | -------------------------------------------------------- |
| `kubectl`                                       | Cluster access; context must be set before running tests |
| [Chainsaw](https://kyverno.github.io/chainsaw/) | `chainsaw test`, `chainsaw lint`                         |
| [Task](https://taskfile.dev/)                   | Wrapper commands in `Taskfile.yml`                       |

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

- Multi-node Kubernetes cluster (minimum 3 nodes including the control plane)
- Descheduler module enabled and deployment ready in `d8-descheduler`
- RBAC permissions to create/delete namespaces, Deployments, Descheduler CRs, and cordon/uncordon nodes
- For faster test cycles, set `deschedulingInterval: Frequent` (5m) in the module ModuleConfig

## Directory Structure

```text
descheduler/
├── Taskfile.yaml              # includes all scenarios
├── chainsaw-config.yaml       # shared timeouts and execution settings
└── tests/
    ├── common/                # shared manifests and assertions
    │   ├── assert-descheduler-ready.yaml
    │   ├── assert-descheduler-rollout-complete.yaml
    │   ├── sts-pinned.yaml
    │   └── sts-unpin-patch.yaml
    ├── low-node-utilization/
    │   ├── chainsaw-test.yaml
    │   ├── manifests/
    │   ├── README.md
    │   └── Taskfile.yml
    ├── high-node-utilization/
    │   └── ...
    ├── exclude-namespaces-from-processing/
    │   └── ...
    ├── statefulset-remove-duplicates/
    │   └── ...
    ├── statefulset-pdb-blocks-eviction/
    │   └── ...
    ├── statefulset-pdb-allows-one-disruption/
    │   └── ...
    ├── statefulset-single-replica-eviction/
    │   └── ...
    └── descheduler-minreplicas-not-supported/
        └── ...
```

Per-scenario details (steps, manifests, expected outcomes): `tests/<name>/README.md`.

## Available Tests

| Task command                                  | Test directory                              | Description                                                    |
| --------------------------------------------- | ------------------------------------------- | -------------------------------------------------------------- |
| `task low-node-utilization:run`               | `tests/low-node-utilization/`               | LowNodeUtilization rebalances pods from overloaded nodes       |
| `task high-node-utilization:run`              | `tests/high-node-utilization/`              | HighNodeUtilization consolidates pods onto fewer nodes         |
| `task exclude-namespaces-from-processing:run` | `tests/exclude-namespaces-from-processing/` | Deckhouse patch prevents eviction of pods in `d8-*` namespaces |
| `task statefulset-remove-duplicates:run` | `tests/statefulset-remove-duplicates/` | StatefulSet without PDB: RemoveDuplicates evicts duplicate pods and they spread across nodes |
| `task statefulset-pdb-blocks-eviction:run` | `tests/statefulset-pdb-blocks-eviction/` | StatefulSet + PDB `maxUnavailable: 0`: every eviction is blocked, pods stay in place |
| `task statefulset-pdb-allows-one-disruption:run` | `tests/statefulset-pdb-allows-one-disruption/` | StatefulSet + PDB `maxUnavailable: 1`: evictions are serialized, StatefulSet stays available |
| `task statefulset-single-replica-eviction:run` | `tests/statefulset-single-replica-eviction/` | Single-replica StatefulSet is evicted — no `minReplicas` protection exists in Deckhouse |
| `task descheduler-minreplicas-not-supported:run` | `tests/descheduler-minreplicas-not-supported/` | `spec.minReplicas` cannot be persisted in the CR; manual ConfigMap edits are overwritten |

## Running Tests

### Validate without a cluster

From a scenario directory:

```bash
cd modules/400-descheduler/e2e/descheduler/tests/low-node-utilization
task dry-run
```

From the suite root (all scenarios):

```bash
cd modules/400-descheduler/e2e/descheduler
task dry-run
```

### Run a single scenario

From a scenario directory:

```bash
cd modules/400-descheduler/e2e/descheduler/tests/low-node-utilization

task run          # full output + JUnit in ./reports/
task run:quiet    # errors and summary only
task run:debug    # pause on failure + fail-fast
```

From the suite root via includes:

```bash
cd modules/400-descheduler/e2e/descheduler

task low-node-utilization:run
task high-node-utilization:run
task exclude-namespaces-from-processing:run
```

### Run all scenarios

```bash
cd modules/400-descheduler/e2e/descheduler
task run
```

### Direct Chainsaw invocation

```bash
cd modules/400-descheduler/e2e/descheduler/tests/low-node-utilization

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
kubectl get deployment -n d8-descheduler
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

Individual steps may poll descheduler logs for one or more descheduling cycles. Plan 15–30 minutes per scenario depending on cluster size and `deschedulingInterval`.

## Reports and Debugging

- JUnit reports: `tests/<scenario>/reports/chainsaw-report.xml` (`reports/` is gitignored)
- On failure, tests collect events and descheduler pod logs from `d8-descheduler`
- Useful manual checks:

```bash
kubectl logs -n d8-descheduler -l app=descheduler --tail=200
kubectl get descheduler -A
kubectl get pods -n d8-descheduler
```
