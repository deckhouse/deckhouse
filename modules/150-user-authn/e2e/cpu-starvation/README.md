# user-authn CPU starvation e2e tests

These end-to-end tests check that Dex of the `user-authn` module survives a node that is short of CPU.
Dex must not fall into a restart loop, must not flood the API server with writes,
and must serve logins again as soon as the pressure ends.
The tests use [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/).

## Overview

Every scenario runs the same in-cluster login load and applies one kind of CPU pressure to the node of the Dex pod.
Each run has four phases, and the checks compare the phases with each other:

- `baseline`: The load runs on an unloaded node.
- `pressure`: The burner pods run on the Dex node for the hold time.
- `after`: The burner is deleted, and the load keeps running for the recovery measurement.
- `drain`: The load stops, and the AuthRequest objects must return to the count before the run.

The scenarios are the following:

| Scenario | Pressure | Dex CPU | Login success under pressure |
| -------- | -------- | ------- | ---------------------------- |
| `dex-noisy-neighbour` | One burner pod requests the free CPU of the node and runs one thread per CPU | Own request (10m–20m without the fix) | Reported only |
| `dex-high-load-average` | Up to 100 burner pods of 3 threads, 60 threads per CPU in total, request the rest of the node | Request raised in place to 500m | At least 80% |
| `dex-high-load-average-cpu-limit` | Same as above | Request and limit of 500m, set in place | Reported only |

The CPU scheduler of Linux (CFS) shares the CPU between cgroups by their weight, which follows the CPU request.
The threads inside one pod compete only with each other,
so one pod with many threads raises the load average but takes no more CPU from Dex than one thread per CPU.
The high load average scenarios therefore spread the threads over many burner pods, each pod with its own cgroup.
Dex competes with every one of them for each slice of its CPU.
The number of burner pods is limited by `maxBurnerPods` and by the free pod slots of the node.

Per-scenario details are in `tests/<NAME>/README.md`.

### In-cluster load

The load runs in the pod `dex-load` of the test namespace, so the result does not depend on the network of the test runner.
The pod runs on a node other than the Dex node, so the pressure starves Dex and not the load generator.
The load goes to the IP address of the Dex pod under test and not to the Service.
In a cluster with several Dex replicas, the other replicas therefore take no part of the load.
The Dex certificate does not cover the pod IP address, so the load does not verify the certificate.

The load consists of the following parts:

- A `DexClient` named `dex-load` with the redirect URI `http://localhost:8000/callback`.
  Deckhouse registers it in Dex as `dex-client-dex-load@<NAMESPACE>` and writes its secret to the Secret `dex-client-dex-load`.
- Two throwaway `User` objects, `e2e-cpu-login-<NAMESPACE>` and `e2e-cpu-refresh-<NAMESPACE>`, with the domain `example.test`.
  Login workers use the first user, refresh workers use the second one.
  With a single user the login workers would evict the refresh tokens through the per-client device limit of Dex.
- A random password per run.
  The init container of the load pod generates it, keeps it in an `emptyDir` volume, and prints only its bcrypt hash.
  The test reads the hash from the log and creates the users with it, so no credential is stored in the repository.
- The container `load` (`tests/common/scripts/dexload.py`).
  It runs the authorization code flow and the refresh token grant against `https://<DEX_POD_IP>:5556`, the HTTPS port of the Dex pod.
  It sends the `Host` header of the issuer, which it reads from the discovery document.
- The container `watch` (`tests/common/scripts/writewatch.py`).
  It watches the leases of `d8-user-authn`, the `useraccounts.deckhouse.io` objects,
  and the `refreshtokens.dex.coreos.com` objects with the ServiceAccount `dex-load`,
  and logs every event to count the writes to the API server.
  It requests only the object metadata (`PartialObjectMetadata`),
  so the refresh tokens of the cluster users never reach the pod.
  A Role in `d8-user-authn` grants the ServiceAccount the watch of the leases and the refresh tokens,
  and a ClusterRole grants the watch of the cluster-scoped `useraccounts` only.

Each finished flow is one line of the `load` container log:

```text
FLOW <START_MS> <DURATION_MS> <login|refresh> <OK_0_OR_1> <DETAIL>
```

The phase marks are taken with the clock of the load pod, so the flows, the writes and the marks share one clock.
The script `tests/common/scripts/flowsum.py` runs in the load pod and turns the log into the success rate per phase.

### Checks

Every scenario runs all checks and fails at the `verdict` step when any of them failed.
The checks are the following:

- **Restarts**: The restart counts of `dex` and `kube-rbac-proxy` of the Dex pod under test do not grow, and the pod is not replaced.
- **Recovery**: Every pod of `d8-user-authn` is Ready within `recovery` seconds after the release, and none is in `CrashLoopBackOff`.
  At least 95% of the logins started from `recovery` to `recovery + 60` seconds after the release succeed.
- **Baseline**: At least 95% of the logins succeed before the pressure. Otherwise, the run proves nothing.
- **Leases**: `holderIdentity` and `leaseTransitions` of the leases in `d8-user-authn` do not change.
- **Write storm**: The `MODIFIED` events under the pressure and in the `after` phase stay within the limit.
  The check fails for a resource only when both thresholds are exceeded at once:
  - The rate exceeds `writeFactor` (2) times the baseline.
    For leases, the rate is the events per minute.
    For `useraccounts` and `refreshtokens`, the rate is the events per completed flow, because these writes follow the logins.
  - The events per minute exceed the baseline plus `writeSlack` (10) events per minute.
- **AuthRequests**: After the load stops, the AuthRequest count returns to the count before the run plus 5 within 16 minutes.
  Dex deletes the AuthRequest of a finished login at once and collects the abandoned ones after their 10-minute expiry.

### Relation to the Dex probe fix

The `dex-noisy-neighbour` scenario checks the fix from the merge request "[user-authn] Keep Dex running on a node short of CPU".
The fix sets 5-second probe timeouts and raises the VPA limit of Dex.

**On code without the fix, the scenario fails at the restart check.**
The reference numbers come from two different runs:

- The manual reproduction of the incident without the fix, with a 15-minute pressure:
  Dex restarted 8 times and kube-rbac-proxy 5 times.
  The 1-second probes timed out, and the restart back-off kept logins down for more than 4 minutes after the pressure ended.
- A run of this scenario on `main`, without the fix, with the default bindings:
  Dex restarted 5 times and kube-rbac-proxy 4 times, and the logins under the pressure failed with "connection refused".
  With the fix, the same scenario passed: no restarts, and the logins recovered to 100% within 2 minutes.

The threshold `minPressureSuccess` of `0.9` in the `dex-high-load-average*` scenarios comes from reference runs
of both scenarios with the fix on a node with 6 CPUs, 64–68 burner pods and the default bindings.
All logins under the pressure succeeded in both runs, with a p95 login time of 1.8 seconds against 1.3 seconds in the baseline.
The 10% margin covers the difference between clusters.

## Prerequisites

This section lists the tools and the cluster that the tests need.

### Tools

The test runner needs the following tools:

| Tool | Purpose |
| ---- | ------- |
| `kubectl` 1.32 or newer | Cluster access, `--subresource resize`; the context must be set before running the tests |
| [Chainsaw](https://kyverno.github.io/chainsaw/) | `chainsaw test`, `chainsaw lint` |
| [Task](https://taskfile.dev/) | Wrapper commands in `Taskfile.yml` |

The runner also needs a POSIX shell with `awk` and `sed`. It does not need ssh or Python.

To install Chainsaw and check the tools, follow the instructions in the [descheduler e2e README](../../../400-descheduler/e2e/descheduler/README.md).

### Cluster requirements

The cluster must meet the following requirements:

- The `user-authn` module is enabled, and the `dex` Deployment in `d8-user-authn` is Available.
- At least two Ready nodes, so the load pod can run outside the Dex node.
- For the `dex-high-load-average*` scenarios, free pod slots on the Dex node: up to 100 burner pods, fewer pods mean fewer threads.
- The nodes can pull `mirror.gcr.io/library/python:3.12-alpine` pinned by digest, or another image set with the `image` binding.
  The image must provide Python 3.12 with the `crypt` module and musl `crypt()` with bcrypt, or the Python `bcrypt` package.
- Kubernetes 1.33 or newer for the `dex-high-load-average*` scenarios, which use in-place pod resize.
- RBAC permissions to create namespaces, pods, Deployments, `DexClient`, cluster-scoped `User`, `ClusterRole` and `ClusterRoleBinding` objects,
  to create `Role` and `RoleBinding` objects in `d8-user-authn`,
  to patch pods, their `resize` subresource and the VPA `dex` in `d8-user-authn`, and to delete pods in `d8-user-authn`.

Run the scenarios on a dedicated cluster.
The burner takes the whole CPU of the Dex node, so every other workload on that node is starved as well.

## Directory structure

The suite has the following layout:

```text
cpu-starvation/
├── Taskfile.yaml              # Includes all scenarios; run, lint, cleanup.
├── chainsaw-config.yaml       # Shared timeouts, execution settings, test namespace label.
└── tests/
    ├── common/
    │   ├── manifests/         # Load pod, RBAC, DexClient, users, burner.
    │   └── scripts/           # lib.sh (step functions), cleanup.sh, dexload.py, writewatch.py, flowsum.py.
    ├── dex-noisy-neighbour/
    ├── dex-high-load-average/
    └── dex-high-load-average-cpu-limit/
```

## Running tests

Each scenario takes 20–40 minutes with the default bindings, most of it in fixed phases and the AuthRequest drain.

### Validating without a cluster

To validate the test files and the configuration against the Chainsaw schema, run the following command:

```bash
cd modules/150-user-authn/e2e/cpu-starvation
task lint
```

The command needs no cluster and prints `The document is valid` for every file.
To validate all e2e tests of the module, run `task lint` in `modules/150-user-authn/e2e`.

### Running a single scenario

To run one scenario, run the following commands:

```bash
cd modules/150-user-authn/e2e/cpu-starvation/tests/dex-noisy-neighbour
kubectl config current-context
task run
```

The JUnit report is written to `./reports/`.

### Overriding bindings

The scenarios read their settings from bindings, and `chainsaw test --set` overrides them.
With Task, pass the flags after `--`.
For example, a manual run with a 15-minute hold and a 3-minute recovery time:

```bash
task run -- --set hold=900 --set recovery=180
```

The bindings are the following (durations are in seconds):

| Binding | Default | Meaning |
| ------- | ------- | ------- |
| `hold` | `300` | Time under pressure; up to 3600 |
| `recovery` | `120` | Time after the release until the pods are Ready and the logins succeed |
| `baseline` | `180` | Load before the pressure |
| `after` | `300` | Load after the release; at least `recovery + 60` |
| `threadsFactor` | `1` (NN), `60` (LA) | Burner threads per node CPU |
| `threadsPerPod` | `0` (NN), `3` (LA) | Threads per burner pod; `0` puts all threads into one pod |
| `maxBurnerPods` | `100` | Upper limit of the burner pods; the free pod slots of the node minus 5 limit them as well |
| `cpuMargin` | `100` | Millicores of the node that the burner leaves unrequested |
| `loginWorkers`, `refreshWorkers`, `pause` | `4`, `4`, `0.5` | Load shape |
| `minPressureSuccess` | `0` (NN), `0.9` (LA, LA limit) | Login success ratio under the pressure; `0` reports only |
| `minRecoveredSuccess` | `0.95` | Login success ratio in the recovery window |
| `writeFactor`, `writeSlack` | `2`, `10` | Write storm limit: factor of the baseline and events per minute above it |
| `authRequestDrain`, `authRequestSlack` | `960`, `5` | AuthRequest drain timeout and tolerance |
| `assertRestarts`, `assertLeases` | `true` | `false` turns the check into a report |
| `dexRequest`, `proxyRequest` | `500m`, `50m` | LA only: in-place CPU requests of `dex` and `kube-rbac-proxy` |
| `image` | `mirror.gcr.io/library/python:3.12-alpine@sha256:b658…` | Image of the load pod and the burner, pinned by digest |

### Running all scenarios

To run all scenarios one after another, run the following commands:

```bash
cd modules/150-user-authn/e2e/cpu-starvation
task run
```

## Cleanup

Chainsaw deletes the test namespace and every object the test applied, including the cluster-scoped users and RBAC objects.
The `dex-high-load-average*` scenarios also restore the VPA update mode of Dex
and delete the resized Dex pod, so the Deployment recreates it from its template.

An interrupted run, for example after Ctrl+C or a lost connection, can leave the following objects behind:

- the test namespace with the burner, the load pod and the `DexClient`;
- the users, the `Role` and `RoleBinding` in `d8-user-authn`, the `ClusterRole` and `ClusterRoleBinding`;
- the VPA `dex` in `updateMode: Off`;
- the resized Dex pod.

Every such object carries the label `e2e-test=user-authn-cpu-starvation`.
Before a run switches the VPA `dex` to `Off`, it saves the previous mode
in the annotation `e2e-test.deckhouse.io/original-update-mode` of the VPA.

To remove the leftovers, run the following commands when no scenario is running:

```bash
cd modules/150-user-authn/e2e/cpu-starvation
kubectl config current-context
task cleanup
```

The task deletes the labelled objects and the labelled Dex pods, waits for the `dex` Deployment to roll out,
and returns the VPA `dex` to the saved mode.
A VPA in `Off` without the annotation gets `InPlaceOrRecreate`, the mode of the module templates.
The task ends with `cleanup done`.
The users also have `ttl: 6h`, so a user left behind expires even without the cleanup.

## Reports and debugging

On failure, the tests collect the pods and events of the test namespace and of `d8-user-authn`,
the last flows of the load, and the Dex logs.
The following commands help with a manual check:

```bash
kubectl -n d8-user-authn get pods -o wide
kubectl -n d8-user-authn get authrequests.dex.coreos.com --no-headers | wc -l
kubectl -n <NAMESPACE> logs dex-load -c load --tail=50
kubectl -n <NAMESPACE> exec dex-load -c load -- python3 -I /scripts/flowsum.py report
```

Here, `<NAMESPACE>` is the test namespace that Chainsaw prints at the start of the run.
