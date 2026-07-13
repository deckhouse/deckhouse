# Control Plane Manager E2E Tests

End-to-end tests for the **control-plane-manager** module, using [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/).

## Overview

These tests verify that control-plane-manager reconciles `ModuleConfig` changes into `ControlPlaneOperation` resources, completes the expected operation pipeline on control-plane nodes, and applies the resulting configuration to static pod manifests.

Each scenario lives in `tests/<name>/` and is executed via Task wrappers that call `chainsaw test` with JUnit reports in `./reports/`.

Both scenarios modify the cluster `control-plane-manager` `ModuleConfig` and restore the original configuration in cleanup. They use `namespace: default` (no ephemeral test namespace) because apiserver restarts can interfere with Chainsaw namespace deletion.

## Chainsaw

[Chainsaw](https://kyverno.github.io/chainsaw/) is a declarative e2e testing tool for Kubernetes. Tests are defined in YAML as a sequence of steps with `try`/`catch`/`cleanup` blocks.

**Key concepts:**

- `try` — main operations; the step fails if any operation fails
- `catch` — runs only on failure (diagnostics collection)
- `cleanup` — runs at test end in reverse step order (used to restore `ModuleConfig`)
- `assert` — polls a resource until it matches or times out

Shared Chainsaw settings are in `chainsaw-config.yaml` at the suite root (`failFast: true`, `parallel: 1`, `delayBeforeCleanup: 15s`, test discovery via `chainsaw-test.yaml`).

## Prerequisites

### Tools

| Tool | Purpose |
| ---- | ------- |
| `kubectl` | Cluster access; context must be set before running tests |
| `jq` | JSON processing in shell helpers |
| `yq` | Required by the `feature-gates` test to read `candi/feature_gates_map.yml` |
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
jq --version
yq --version
```

### Cluster requirements

- `control-plane-manager` module enabled and healthy
- RBAC permissions to read/write `ModuleConfig`, read `ControlPlaneOperation`, and read control-plane-manager logs
- Control-plane nodes with running static pods for the components under test (kube-apiserver; for `feature-gates` also kube-controller-manager and kube-scheduler)

## Directory Structure

```text
control-plane-manager/
├── Taskfile.yaml              # includes all scenarios
├── chainsaw-config.yaml       # shared timeouts and execution settings
├── functions.sh               # shared kubectl/CPO helpers
└── tests/
    ├── basic-audit-policy/
    │   ├── chainsaw-test.yaml
    │   ├── basic_audit_policy.md
    │   ├── manifests/
    │   ├── scripts/
    │   │   └── functions.sh   # symlink to ../../functions.sh
    │   └── Taskfile.yml
    └── feature-gates/
        ├── chainsaw-test.yaml
        ├── feature_gates.md
        ├── manifests/         # example only; runtime manifest is generated
        ├── scripts/
        │   ├── functions.sh   # symlink to ../../functions.sh
        │   └── feature-gates.sh
        └── Taskfile.yml
```

Per-scenario details: `tests/<name>/<name>.md`.

## Shared Helpers

`functions.sh` at the suite root provides helpers used by all scenarios (symlinked from each test's `scripts/`):

- `kubectl_run` — waits for the API and retries kubectl on transient or conflict errors
- `wait_until`, `wait_for_api` — polling and API readiness
- `backup_moduleconfig_spec` / `restore_moduleconfig` — backup and restore `ModuleConfig` spec (JSON patch replace)
- `snapshot_component_cpos`, `wait_for_new_component_cpo`, `wait_for_new_control_plane_cpos` — ControlPlaneOperation tracking
- `apply_or_patch_moduleconfig`, `is_flag_in_component`, `kubernetes_version`

The `feature-gates` scenario adds `scripts/feature-gates.sh` for reading `candi/feature_gates_map.yml` and building `enabledFeatureGates` dynamically.

## Available Tests

| Task command | Test directory | Description |
| ------------ | -------------- | ----------- |
| `task basic-audit-policy:run` | `tests/basic-audit-policy/` | Sets `basicAuditPolicyEnabled: false`, verifies a new kube-apiserver `ControlPlaneOperation` completes and audit policy is removed from manifests |
| `task feature-gates:run` | `tests/feature-gates/` | Enables all supported feature gates for the cluster Kubernetes version, verifies CPOs on apiserver/controller-manager/scheduler complete and gates appear in manifests |

## Running Tests

### Validate without a cluster

From a scenario directory:

```bash
cd modules/040-control-plane-manager/e2e/control-plane-manager/tests/basic-audit-policy
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
cd modules/040-control-plane-manager/e2e/control-plane-manager/tests/basic-audit-policy

task run          # full output + JUnit in ./reports/
task run:verbose  # verbose chainsaw output
task run:debug    # pause on failure + fail-fast
task lint:test    # validate chainsaw-test.yaml
```

From the suite root via includes:

```bash
cd modules/040-control-plane-manager/e2e/control-plane-manager
task basic-audit-policy:run
task feature-gates:run
```

### Run all scenarios

```bash
cd modules/040-control-plane-manager/e2e/control-plane-manager
task run
```

Scenarios run sequentially (`parallel: 1`) because they both modify the same cluster `ModuleConfig`.

### Direct Chainsaw invocation

```bash
cd modules/040-control-plane-manager/e2e/control-plane-manager/tests/basic-audit-policy

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
kubectl get controlplaneoperations -n kube-system
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

Individual script steps may define their own timeouts (for example 5–15 minutes while waiting for new operations). Plan ~15 minutes for `basic-audit-policy` and longer for `feature-gates` (three components).

## Reports and Debugging

- JUnit reports: `tests/<name>/reports/chainsaw-report.xml` (`reports/` is gitignored)
- Useful manual checks:

```bash
kubectl get moduleconfig control-plane-manager -o yaml
kubectl get controlplaneoperations -n kube-system -o wide
kubectl get pods -n kube-system -l 'component in (kube-apiserver,kube-controller-manager,kube-scheduler)'
kubectl logs -n kube-system -l app=d8-control-plane-manager --tail=100
```

If cleanup did not run (for example the test process was killed), restore manually:

```bash
BACKUP_FILE="${TMPDIR:-/tmp}/cpm-e2e-moduleconfig-backup.json"
cd modules/040-control-plane-manager/e2e/control-plane-manager/tests/basic-audit-policy
. ./scripts/functions.sh && restore_moduleconfig "$BACKUP_FILE"
```

## Safety

These tests modify the cluster `control-plane-manager` `ModuleConfig`, which triggers real control plane reconciliations. Expect brief apiserver (and, for `feature-gates`, controller-manager and scheduler) restarts during a run. Cleanup restores the backed-up configuration at test end.
