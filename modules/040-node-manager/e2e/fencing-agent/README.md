# Fencing Agent E2E Tests

End-to-end tests for the **fencing-agent** component of the `node-manager` module, using [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/).

## Overview

These tests verify that fencing-agent **deploys** and **runs** when `spec.fencing` is enabled on a NodeGroup, and that the DaemonSet is removed after fencing is disabled.

Each scenario lives in `tests/<name>/` and is executed via Task wrappers that call `chainsaw test` with JUnit reports in `./reports/`.

## Chainsaw

[Chainsaw](https://kyverno.github.io/chainsaw/) is a declarative e2e testing tool for Kubernetes. Tests are defined in YAML as a sequence of steps with `try`/`catch`/`finally` blocks. Chainsaw applies cluster-scoped resources, runs assertions and scripts, and cleans up automatically.

**Key concepts:**

- `try` — main operations; the step fails if any operation fails
- `catch` — runs only on failure (diagnostics collection)
- `cleanup` — runs after the step completes (resource deletion)
- `$NAMESPACE` — auto-generated test namespace, available in scripts (not used in current scenarios)

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

- NodeGroup named `worker` with at least one schedulable node
- Worker nodes support the `softdog` kernel module
- Fencing-agent image built and available in the Deckhouse registry
- RBAC permissions to patch NodeGroups and read DaemonSets, pods, and nodes

## Directory Structure

```text
fencing-agent/
├── Taskfile.yaml              # includes all scenarios
├── chainsaw-config.yaml       # shared timeouts and execution settings
└── tests/
    └── fencing-agent-deployment/
        ├── chainsaw-test.yaml
        ├── manifests/
        ├── asserts/
        ├── fencing_agent_deployment.md
        └── Taskfile.yml
```

Per-scenario details (steps, manifests, expected outcomes): `tests/fencing-agent-deployment/fencing_agent_deployment.md`.

## Available Tests

| Task command                        | Test directory                    | Description                                                                                    |
| ----------------------------------- | --------------------------------- | ---------------------------------------------------------------------------------------------- |
| `task fencing-agent-deployment:run` | `tests/fencing-agent-deployment/` | Enables Watchdog fencing on `worker`, verifies DaemonSet and node labels, then reverts fencing |

## Running Tests

### Validate without a cluster

From a scenario directory:

```bash
cd modules/040-node-manager/e2e/fencing-agent/tests/fencing-agent-deployment
task dry-run
```

From the suite root:

```bash
cd modules/040-node-manager/e2e/fencing-agent
task dry-run
```

### Run a single scenario

From a scenario directory:

```bash
cd modules/040-node-manager/e2e/fencing-agent/tests/fencing-agent-deployment

task run          # full output + JUnit in ./reports/
task run:verbose  # verbose chainsaw output
task run:debug    # pause on failure + fail-fast
```

From the suite root via includes:

```bash
cd modules/040-node-manager/e2e/fencing-agent
task fencing-agent-deployment:run
```

### Run all scenarios

```bash
cd modules/040-node-manager/e2e/fencing-agent
task run
```

### Direct Chainsaw invocation

```bash
cd modules/040-node-manager/e2e/fencing-agent/tests/fencing-agent-deployment

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
kubectl get nodegroup worker
kubectl get nodes -l node.deckhouse.io/group=worker
```

## Timeouts

From `chainsaw-config.yaml`:

| Timeout | Value |
| ------- | ----- |
| apply   | 30s   |
| assert  | 5m    |
| error   | 2m    |
| delete  | 30s   |
| cleanup | 2m    |
| exec    | 30s   |

The scenario waits up to 5 minutes for the `fencing-agent-worker` DaemonSet to become ready and up to 2 minutes for deletion after fencing is disabled. Plan ~10 minutes for a full run.

## Reports and Debugging

- JUnit reports: `tests/fencing-agent-deployment/reports/chainsaw-report.xml` (`reports/` is gitignored)
- Useful manual checks:

```bash
kubectl get daemonset fencing-agent-worker -n d8-cloud-instance-manager
kubectl get pods -n d8-cloud-instance-manager -l app=fencing-agent
kubectl get nodes -l node-manager.deckhouse.io/fencing-enabled
```
