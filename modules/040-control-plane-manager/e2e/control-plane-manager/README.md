# Control Plane Manager E2E Tests

End-to-end tests for the **control-plane-manager** module, using [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/).

## Overview

These tests verify that control-plane-manager reconciles configuration changes into `ControlPlaneOperation` resources and completes the expected operation pipeline on control-plane nodes.

Each scenario lives in `tests/<name>/` and is executed via Task wrappers that call `chainsaw test` with JUnit reports in `./reports/`.

## Chainsaw

[Chainsaw](https://kyverno.github.io/chainsaw/) is a declarative e2e testing tool for Kubernetes. Tests are defined in YAML as a sequence of steps with `try`/`catch`/`finally` blocks. Chainsaw applies cluster-scoped resources, runs assertions and scripts, and cleans up automatically.

**Key concepts:**

- `try` — main operations; the step fails if any operation fails
- `catch` — runs only on failure (diagnostics collection)
- `cleanup` — runs after the step completes (resource deletion or restoration)
- `$NAMESPACE` — auto-generated test namespace (not used in current scenarios)

Shared Chainsaw settings are in `chainsaw-config.yaml` at the suite root (`failFast: true`, `parallel: 1`, test discovery via `chainsaw-test.yaml`).

## Prerequisites

### Tools

| Tool | Purpose |
| ---- | ------- |
| `kubectl` | Cluster access; context must be set before running tests |
| [Chainsaw](https://kyverno.github.io/chainsaw/) | `chainsaw test`, `chainsaw lint` |
| [Task](https://taskfile.dev/) | Wrapper commands in `Taskfile.yml` |

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

- `control-plane-manager` module enabled and healthy
- RBAC permissions to read/write `ModuleConfig`, read `ControlPlaneOperation`, and read control-plane-manager logs
- A working kube-apiserver static pod on at least one control-plane node

## Directory Structure

```text
control-plane-manager/
├── Taskfile.yaml              # includes all scenarios
├── chainsaw-config.yaml       # shared timeouts and execution settings
└── tests/
    └── apiserver-operation/
        ├── chainsaw-test.yaml
        ├── manifests/
        ├── apiserver_operation.md
        └── Taskfile.yml
```

Per-scenario details (steps, manifests, expected outcomes): `tests/apiserver-operation/apiserver_operation.md`.

## Available Tests

| Task command | Test directory | Description |
| ------------ | -------------- | ----------- |
| `task apiserver-operation:run` | `tests/apiserver-operation/` | Disables basic audit policy, verifies a new kube-apiserver `ControlPlaneOperation` completes with the expected steps |

## Running Tests

### Validate without a cluster

From a scenario directory:

```bash
cd modules/040-control-plane-manager/e2e/control-plane-manager/tests/apiserver-operation
task dry-run
```

From the suite root:

```bash
cd modules/040-control-plane-manager/e2e/control-plane-manager
task dry-run
```

### Run a single scenario

From a scenario directory:

```bash
cd modules/040-control-plane-manager/e2e/control-plane-manager/tests/apiserver-operation

task run          # full output + JUnit in ./reports/
task run:verbose  # verbose chainsaw output
task run:debug    # pause on failure + fail-fast
```

From the suite root via includes:

```bash
cd modules/040-control-plane-manager/e2e/control-plane-manager
task apiserver-operation:run
```

### Run all scenarios

```bash
cd modules/040-control-plane-manager/e2e/control-plane-manager
task run
```

### Direct Chainsaw invocation

```bash
cd modules/040-control-plane-manager/e2e/control-plane-manager/tests/apiserver-operation

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
kubectl get moduleconfig control-plane-manager
kubectl get controlplaneoperations -n kube-system -l control-plane.deckhouse.io/component=kube-apiserver
```

## Timeouts

From `chainsaw-config.yaml`:

| Timeout | Value |
| ------- | ----- |
| apply | 30s |
| assert | 10m |
| error | 2m |
| delete | 30s |
| cleanup | 5m |
| exec | 30s |

The scenario waits up to 5 minutes for a new `ControlPlaneOperation` to appear and up to 10 minutes for it to complete. Plan ~15 minutes for a full run.

## Reports and Debugging

- JUnit reports: `tests/apiserver-operation/reports/chainsaw-report.xml` (`reports/` is gitignored)
- Useful manual checks:

```bash
kubectl get moduleconfig control-plane-manager -o yaml
kubectl get controlplaneoperations -n kube-system -l control-plane.deckhouse.io/component=kube-apiserver
kubectl logs -n kube-system -l app=d8-control-plane-manager --tail=100
```
