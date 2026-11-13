# Fencing Agent E2E Tests

End-to-end tests for the **fencing-agent** component of the `node-manager` module, using [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/).

## Overview

These tests verify the fencing 2.0 agents on a NodeGroup that the test creates itself: the agent DaemonSet, its ConfigMap, the key Secret, the NodeGroupConfigurations, the node condition `Fencing` and the profile annotations in the modes `Normal`, `Slow`, `Critical`, the deprecated `Watchdog` and `Disabled`. A separate scenario covers Deckhouse Engine (`systemType: Immutable`) groups, where no bashible runs: the built-in softdog, the timeout the agent applies itself, the condition `Fencing` and the profile annotations, and `kernel.panic=0` published through NodeConfig.

Each scenario lives in `tests/<name>/` and is executed via Task wrappers that call `chainsaw test` with JUnit reports in `./reports/`.

## Chainsaw

[Chainsaw](https://kyverno.github.io/chainsaw/) is a declarative e2e testing tool for Kubernetes. Tests are defined in YAML as a sequence of steps with `try`/`catch`/`finally` blocks. Chainsaw applies cluster-scoped resources, runs assertions and scripts, and cleans up automatically.

**Key concepts:**

- `try` — main operations; the step fails if any operation fails
- `catch` — runs only on failure (diagnostics collection)
- `finally` — runs at the end of the step
- `cleanup` — runs at the end of the test, in reverse step order, also after a failure
- bindings — values such as `$nodeGroup` that operations and JMESPath expressions use; `$namespace` is the auto-generated test namespace and `$client` gives the `x_k8s_*` functions access to the cluster
- step templates (`StepTemplate`, `use:`) — a step defined once and called with its own bindings

The `fencing-agent-deployment` scenario uses only Chainsaw operations (`apply`, `patch`, `assert`, `error`, `delete`, `wait`, `command`) and no shell scripts. Every change and every check is a step of its own, so the JUnit report (`JUNIT-OPERATION`, one entry per operation) shows each check separately.

Shared Chainsaw settings are in `chainsaw-config.yaml` at the suite root (`failFast: true`, `parallel: 1`, test discovery via `chainsaw-test.yaml`, a `JUNIT-OPERATION` report in `reports/`).

## Prerequisites

### Tools

| Tool                                            | Purpose                                                  |
| ----------------------------------------------- | -------------------------------------------------------- |
| `kubectl`                                       | Cluster access; context must be set before running tests |
| [Chainsaw](https://kyverno.github.io/chainsaw/) | `chainsaw test`, `chainsaw lint`                         |
| [Task](https://taskfile.dev/)                   | Wrapper commands in `Taskfile.yaml`                      |

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

- For `fencing-agent-deployment`: a `CloudEphemeral` NodeGroup (`FENCING_SOURCE_NODE_GROUP`, default `worker`) that gives the instance class and the zone; the test creates its own group `FENCING_TEST_NODE_GROUP` (default `e2e-fencing`) and never patches the source group
- A commercial edition of Deckhouse: both scenarios set the `Critical` mode, and Community Edition (CE) does not let you set it
- Nodes support the `softdog` kernel module
- For `fencing-agent-engine` only: `jq` and a NodeGroup with `systemType: Immutable` named by `FENCING_ENGINE_NODE_GROUP`, on a Deckhouse Engine release with softdog built in (see `tests/fencing-agent-engine/fencing_agent_engine.md`)
- Fencing-agent image built and available in the Deckhouse registry
- RBAC permissions to create and delete NodeGroups, annotate nodes, read DaemonSets, ConfigMaps, NodeGroupConfigurations, Machines, Instances, pods and nodes, check that a Secret exists, and delete Projects when the module `multitenancy-manager` is on

## Directory Structure

```text
fencing-agent/
├── Taskfile.yaml              # includes all scenarios
├── chainsaw-config.yaml       # shared timeouts, execution and report settings
├── .gitignore                 # reports/
└── tests/
    ├── fencing-agent-deployment/
    │   ├── chainsaw-test.yaml
    │   ├── manifests/
    │   ├── asserts/
    │   ├── step-templates/
    │   ├── fencing_agent_deployment.md
    │   └── Taskfile.yaml
    └── fencing-agent-engine/
        ├── chainsaw-test.yaml
        ├── manifests/
        ├── asserts/
        ├── scripts/
        ├── fencing_agent_engine.md
        └── Taskfile.yaml
```

Per-scenario details (steps, manifests, expected outcomes): `tests/<name>/<name>.md`.

## Available Tests

| Task command                        | Test directory                    | Description                                                                                    |
| ----------------------------------- | --------------------------------- | ---------------------------------------------------------------------------------------------- |
| `task fencing-agent-deployment:run` | `tests/fencing-agent-deployment/` | Creates a test NodeGroup, switches it through `Normal`, `Slow`, `Critical`, `Watchdog` and `Disabled`, checks the agent objects, the condition `Fencing`, the annotations and `kernel.panic`, then deletes the group (`FENCING_TEST_NODE_GROUP`, `FENCING_SOURCE_NODE_GROUP`, `FENCING_INSTANCE_CLASS`) |
| `task fencing-agent-engine:run`     | `tests/fencing-agent-engine/`     | On a Deckhouse Engine group: built-in softdog, profile timeout applied by the agent, `kernel.panic=0` via NodeConfig, disarm on maintenance and restart, revert. Not part of `task run`: needs an Engine group (`FENCING_ENGINE_NODE_GROUP`) |

## Running Tests

### Validate without a cluster

`task lint:test` and `task lint:configuration` check the files against the Chainsaw schema. `task dry-run` (`--no-cluster`) loads the test and the step templates, but with Chainsaw 0.2.15 it fails on the first operation that needs the cluster, so it is no check of the scenario.

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
task run:quiet    # errors, failures and the summary only
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
  --report-format JUNIT-OPERATION \
  --report-path ./reports/
```

### kubectl context

Chainsaw uses the current `kubectl` context. Before running:

```bash
kubectl config current-context
kubectl get nodegroup "${FENCING_SOURCE_NODE_GROUP:-worker}"
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

The `fencing-agent-deployment` scenario sets longer timeouts on single operations: 30 minutes for the new cloud node, 20 minutes for the delete of the group, 15 minutes for each check that nothing of the group is left, 10 minutes for `kernel.panic`. Plan ~40 minutes for a full run.

## Reports and Debugging

- JUnit reports: `chainsaw-config.yaml` writes a `JUNIT-OPERATION` report to `reports/chainsaw-report.xml` of the directory Chainsaw runs in, also without the Task wrappers; `--report-*` flags override it (`reports/` is gitignored)
- Useful manual checks:

```bash
kubectl get daemonsets -n d8-cloud-instance-manager -l app=fencing-agent
kubectl get pods -n d8-cloud-instance-manager -l app=fencing-agent
kubectl get nodegroup e2e-fencing
```
