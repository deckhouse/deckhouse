# CPU starvation e2e tests

This suite checks how the `admission-policy-engine` module behaves when the node it runs on is short of CPU.
The tests use [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) and need only `kubectl` access to the cluster.

## Overview

Every scenario puts one kind of CPU pressure on the node of a Gatekeeper pod and asserts three properties:

- the module keeps doing its job, maybe slower: the webhook allows a compliant Pod and denies a privileged one;
- the module neither restarts nor floods the API server with writes;
- the module recovers as soon as the pressure ends.

The suite applies two pressure modes, and both rely on how the Completely Fair Scheduler (CFS) divides the CPU.
CFS shares the CPU between cgroups by weight, and the weight of a pod follows its CPU request.
Threads inside one cgroup compete only with each other:
they take no larger share from another cgroup and barely delay it.

In the *noisy neighbour* mode, one Burstable stress pod requests all free CPU of the node
and runs one burner thread per CPU.
The module keeps its own small request, so under contention it gets a small share of the CPU by weight.

In the *high load average* mode, the CPU request of the target container is first raised in place to its real usage,
so the module keeps the CPU share it needs.
Then many small Burstable stress pods request the rest of the node, and each runs 2 to 4 burner threads.
Every stress pod is a cgroup of its own that competes with the module for the run queue,
so the module waits longer for every wake-up, and the load average of the node grows with the number of threads.
A variant also sets a CPU limit equal to the request, which adds CFS quota throttling.

Each scenario lives in `tests/<name>/`.
All scenarios share the script `tests/common/scripts/cpu-starvation.sh` and the manifests in `tests/common/manifests/`.

## How a scenario works

A scenario runs the same sequence of Chainsaw steps; each step calls one subcommand of the shared script:

1. `preflight` checks that the module is healthy and that the test namespace is served by the webhook.
   It first labels the namespace `security.deckhouse.io/pod-policy=baseline`
   and `security.deckhouse.io/pod-policy-action=deny`,
   so the Baseline Pod Security Standard denies a privileged Pod there whatever the cluster default is.
   Deckhouse renders the `d8-pod-security-baseline-deny*` constraints only while some namespace uses the deny action,
   so the step then waits up to 5 minutes for such a constraint and up to 2 minutes for the probes to pass.
1. `freeze-vpa` switches the VerticalPodAutoscaler (VPA) of the target to `updateMode: Off`,
   so its updater neither evicts nor resizes the target during the test.
1. `start-observers` takes snapshots and starts background observers (see below).
1. `baseline` collects the baseline without pressure for `baselineWindow` and ends it with the `baseline-end` mark.
   It fails before any pressure when the probes are broken: a probe infrastructure error, an unreachable API server,
   or less than half of the probes answered as expected.
1. In the high load average scenarios, `resize` raises the CPU request of the `manager` container in place
   with `kubectl patch --subresource resize`.
1. `stress-start` computes the free CPU of every target node as allocatable CPU minus the requests of the pods
   on the node minus 100m, and starts a stress Deployment pinned to the node that requests it.
   The step sets the `pressure` mark once every stress pod runs.
1. `hold` keeps the pressure for `hold`.
1. `release` deletes the stress Deployments and waits up to `recovery` for the module to recover.
1. `after` keeps observing for `afterWindow`
   and waits up to `auditRecovery` for an audit cycle that ends after the release.
1. `analyse` stops the observers, prints a summary and fails the test on any violated assertion.

The Chainsaw cleanup of the test deletes the stress Deployments, restores the resized container and the VPA update mode,
and stops the observers.

### Phases

The `analyse` step splits the observations into phases by the marks in the `phases` file:

- the baseline lasts from the start of the observers to the `baseline-end` mark;
- the ramp-up lasts from the `ramp-up` mark to the `pressure` mark.
  The `resize` step sets the `ramp-up` mark when it starts, or the `stress-start` step when there is no resize.
  The ramp-up covers the in-place resize, the creation of the stress pods and their rollout;
- the pressure lasts from the `pressure` mark to the `release` mark;
- the after-release phase lasts from the `recovered` mark to the `end` mark for the probes
  and from the `release` mark to the `end` mark for the write rates.

The ramp-up never counts as the baseline.
The probe budget and the write storm check apply to it as to the pressure,
but the minimum number of probes does not, because the ramp-up can be short.

### Stress pods

The stress Deployment of a node depends on the mode.

In the noisy neighbour mode, the Deployment has one replica that requests all free CPU
and runs `threadsFactor` (1) burner threads per CPU.

In the high load average mode, the Deployment has many replicas whose requests add up to the free CPU.
The number of replicas is the smallest of the following values:

- `maxStressPods` (100 by default);
- the free pod slots of the node: `status.allocatable.pods` minus the pods on the node minus a reserve of 10,
  so the module and DaemonSets can still start pods there;
- the free CPU divided by 10m, the smallest request of one stress pod;
- the number of pods that run `threadsFactor` (60) threads per CPU with 2 threads each.

Every replica then runs 2 to 4 threads, as close to `threadsFactor` threads per CPU in total as the limit of 4 allows.
On a node with 4 CPU, 3 of them free and 70 free pod slots, the step starts 70 pods with 4 threads each:
280 threads, about 40m requested per pod.
The `stress-start` step prints the pods, threads and requests it chose for every node.

More pods mean more cgroups that compete with the module for the run queue,
so the layout prefers many pods with few threads to few pods with many threads.
The limit of 100 pods keeps the kubelet of the node, often a master, able to start them in a few minutes.

### Background observers

The background observers record the following data:

- the admission probes: a loop of `kubectl create --dry-run=server` of a compliant Pod (must be allowed)
  and of a privileged Pod (must be denied by a `d8-pod-security-baseline-deny*` constraint),
  about 2 requests per second;
- the watch events of `leases`, `constraintpodstatuses.status.gatekeeper.sh` and
  `constrainttemplatepodstatuses.status.gatekeeper.sh` in `d8-admission-policy-engine`;
- the presence of the ValidatingWebhookConfiguration `d8-admission-policy-engine-config`, polled every 2 seconds.

Chainsaw gives every script step a temporary kubeconfig and deletes it when the step ends.
The observers outlive the step, so `start-observers` copies the effective kubeconfig
(`kubectl config view --raw --minify --flatten`) to a private temporary directory
(`$TMPDIR/ape-cpu-starvation.XXXXXX`, mode `0700`) outside `reports/`.
The state directory keeps only the path of the copy, in `kubeconfig-path`.
`stop-observers` deletes the copy, because it holds credentials.

The probes classify every answer:

- `ok` and `denied` are the expected admission answers;
- `allowed` means the privileged Pod was admitted;
- `error` means the API server answered, but admission failed in the module,
  for example a call to a webhook of the module failed or timed out;
- `foreign-webhook` means a call to a webhook of another module or application failed, timed out,
  or that webhook denied the request;
- `unreachable` means the API server did not take the request: the connection was refused or reset,
  the client timed out, or the server answered `429 Too Many Requests`;
- `infra` means the probe itself is broken: a missing kubeconfig, authentication or RBAC errors,
  a wrong certificate or host name.

Only `error` counts toward the admission failure budget.
In the ramp-up, under pressure and after the release, `unreachable` answers have a budget of their own of the same size,
and the summary reports them as "API server unreachable under pressure":
the node under pressure often runs kube-apiserver too,
so this is a finding about the control plane, not about the module.
Any `infra` result fails the test as a broken probe infrastructure.
In the baseline, both `unreachable` and `infra` fail the test before the pressure starts.
The first answer of each class is saved in full.
The probes run with `--validate=false`, so a failed OpenAPI schema download cannot break them.

The probes take the webhook name from `failed calling webhook "<NAME>"` or `admission webhook "<NAME>"`
in the `kubectl` output and write it to the probe log as `webhook=<NAME>`.
A failure counts as `error` only for the webhooks of the `d8-admission-policy-engine-config`
ValidatingWebhookConfiguration and MutatingWebhookConfiguration, listed in `MODULE_WEBHOOKS` of the script.
The list follows `templates/validatingwebhookconfiguration.yaml` and `templates/mutatingwebhookconfiguration.yaml`
of the module, and `preflight` warns about a webhook of these configurations that is missing from it.
An answer that mentions a webhook without its name counts as `error`.

The `foreign-webhook` results never fail the test.
The summary shows their count in every phase and lists the webhooks with the number of failed probes,
and the final verdict repeats the list as a note.
For example, a webhook of another module with `failurePolicy: Fail` that times out under pressure
appears there instead of the admission failures of the module.

### Interrupted runs

Chainsaw does not run the cleanup when its process is killed, for example with `Ctrl+C`.
The observers then stop by themselves at a deadline: the planned end of the remaining steps plus 10 minutes.
Every long step moves the deadline, so the margin covers only one step that overruns.

When the deadline passes without `stop-observers`, a background reaper deletes the stress Deployments
and restores the VPA update mode, and the observers delete the kubeconfig copy.
The resized container, the namespace label and the test namespace stay; remove them with `task cleanup`
(see [Cleanup after an interrupted run](#cleanup-after-an-interrupted-run)).

## Assertions

The `analyse` step fails the test when any of the following conditions holds:

| Check | Fails when |
| ----- | ---------- |
| Admission | Probes with the `error` result exceed `maxFailurePercent` (1% by default) of all probes in a phase |
| Probe count | The pressure or the after-release phase has fewer than 30 allowed probes |
| API server availability | `unreachable` probes exceed `maxFailurePercent` in the ramp-up, under pressure or after the release |
| Deny path | The privileged Pod is admitted at least once in any phase |
| Recovery | The module pods are not Ready, or the probes do not pass, within `recovery` after the release |
| Write storm | The rate of MODIFIED events in the ramp-up, under pressure or after the release is more than 2x the baseline **and** more than baseline + 10 per minute |
| Webhook configuration | `d8-admission-policy-engine-config` is missing, recreated with another UID, or has fewer webhooks at any poll |
| Webhook configuration poll | Polls without an answer (API errors or an unreachable API server) exceed `maxFailurePercent` |
| Observers | Any probe or poll has the `infra` result, less than half of the baseline probes succeed, or a watch fails more than 10 times |
| Restarts | A container restart count of a module pod increased, or a module pod was replaced |
| Leases | The holder or the transition count of a lease in `d8-admission-policy-engine` changed |
| Audit | No audit cycle completed within `auditRecovery` after the release |

Gatekeeper does not use leader election, so the namespace normally has no leases.
The lease checks then pass trivially; they guard against a future change.

The audit check reads `gatekeeper_audit_last_run_end_time` from the audit pod through the API server pod proxy.
When the metric is unavailable, it uses the newest `status.auditTimestamp` of the constraints.

The summary also prints informational numbers that do not fail the test:
probe latency (p50 and p95, including the `kubectl` start-up time),
the CPU busy on the node and used by the target container under pressure,
and the share of time the container waited for CPU
(`container_pressure_cpu_waiting_seconds_total`, when the kubelet exports it).
Bash 5 measures the latency with `EPOCHREALTIME`, bash 3.2 with the `Time::HiRes` module of `perl`.
Without either, the latency is recorded as `NA` and printed as `n/a`.

## Reference results

The `webhook-noisy-neighbour` scenario passed with the default settings on a cluster
with one master and one webhook replica:

- 0 failed probes before, during and after the pressure;
- p95 probe latency of 3.2 seconds under the pressure and 0.4 seconds without it;
- the webhook waited for CPU 35% of the time;
- no restarts, and an audit cycle completed 15 seconds after the pressure.

The other scenarios have not been run yet.

A manual check made before the scenarios were written is a separate data point.
On a master with 8 CPU, 4 of them requested, a neighbour ran 8 burner threads.
The Gatekeeper webhook had no failed requests and no restarts, with a p95 latency of 1.2–2.3 seconds.
The requests were sent by hand, not by the scenario probes, so the numbers are not directly comparable.

## Prerequisites

The tests run against the current `kubectl` context and need the following tools:

| Tool | Purpose |
| ---- | ------- |
| `kubectl` 1.32 or newer | Cluster access, `patch --subresource resize` in the high load average scenarios |
| [Chainsaw](https://kyverno.github.io/chainsaw/) | `chainsaw test`, `chainsaw lint` |
| [Task](https://taskfile.dev/) | Wrapper commands in `Taskfile.yml` |
| `bash`, `awk`, `sed`, `pgrep`, `mktemp` | The shared scenario script; bash 3.2 is enough |
| `perl` with `Time::HiRes` | Optional: millisecond probe latency with bash 3.2 |

The cluster must meet the following requirements:

- the `admission-policy-engine` module is enabled, and `gatekeeper-controller-manager` and `gatekeeper-audit` are Ready;
- the module renders a `d8-pod-security-baseline-deny*` constraint of the `D8PrivilegedContainer` kind
  with `enforcementAction: deny` once the test namespace has the deny action label;
- the stress image (`mirror.gcr.io/library/busybox:1.36` by default) can be pulled on the target nodes;
- the target nodes have room for the stress pods: free pod slots and 4Mi of memory per pod;
- the metrics API (`kubectl top`) works, so the high load average scenarios can read the container usage;
- Kubernetes 1.33 or newer for the high load average scenarios (in-place Pod resize);
- the user may create Deployments, label namespaces,
  annotate and patch pods and VPA objects in `d8-admission-policy-engine`,
  patch the `resize` subresource of pods, restart Deployments in `d8-admission-policy-engine`,
  and read `pods/proxy` and `nodes/proxy`.

**The scenarios load whole nodes, often a master node, and briefly change module resources.**
Run them only on a test cluster and never in parallel with each other.

## Directory structure

The suite has the following layout:

```text
cpu-starvation/
├── Taskfile.yaml              # includes all scenarios
├── chainsaw-config.yaml       # shared timeouts and execution settings
└── tests/
    ├── common/
    │   ├── manifests/         # admission probes and the stress Deployment template
    │   └── scripts/
    │       └── cpu-starvation.sh
    ├── webhook-noisy-neighbour/
    │   ├── chainsaw-test.yaml
    │   ├── README.md
    │   └── Taskfile.yml
    ├── audit-noisy-neighbour/
    ├── webhook-high-load-average/
    └── webhook-high-load-average-cpu-limit/
```

## Available tests

The suite contains the following scenarios:

| Task command | Target | Pressure |
| ------------ | ------ | -------- |
| `task webhook-noisy-neighbour:run` | `gatekeeper-controller-manager` | One stress pod with all free CPU, 1 thread per CPU |
| `task audit-noisy-neighbour:run` | `gatekeeper-audit` | One stress pod with all free CPU, 1 thread per CPU |
| `task webhook-high-load-average:run` | `gatekeeper-controller-manager` | Request raised to usage; up to 100 stress pods, up to 60 threads per CPU |
| `task webhook-high-load-average-cpu-limit:run` | `gatekeeper-controller-manager` | As above, plus a CPU limit equal to the request |

When the webhook runs in several replicas, the stress pods are started on the node of every replica.

## Settings

Every setting is a Chainsaw binding with a short default.
Override a binding with `--set <name>=<value>` after `--` in a `task run` command.

| Binding | Default | Meaning |
| ------- | ------- | ------- |
| `hold` | `5m` | How long the pressure lasts; use `15m` for manual runs |
| `recovery` | `120s` | Recovery budget after the release |
| `baselineWindow` | `3m` | Baseline observation time before the pressure |
| `afterWindow` | `5m` | Observation time after the release |
| `auditRecovery` | `5m` | Time after the release within which an audit cycle must complete |
| `threadsFactor` | `1` or `60` | Burner threads per node CPU: 1 for noisy neighbour, target of 60 for high load average |
| `maxStressPods` | `100` | High load average only: the most stress pods per node |
| `stressImage` | `mirror.gcr.io/library/busybox:1.36` | Image with `/bin/sh` and `seq` for the stress pods |
| `maxFailurePercent` | `1` | Budget in percent for failed probes, unreachable probes and unanswered polls |
| `probeInterval` | `0.5` | Pause between two probe pairs, in seconds |

Durations accept an integer with an optional `s`, `m` or `h` suffix.
The step timeouts allow `hold` and `afterWindow` up to about one hour.

## Running tests

Validate the tests without a cluster from the suite root:

```bash
cd modules/015-admission-policy-engine/e2e/cpu-starvation
task dry-run
```

The command runs `chainsaw lint` for every test and the configuration, and `bash -n` for the shared script.
The scenario steps are scripts that need a cluster, so `chainsaw test --no-cluster` cannot run them.

Run one scenario from its directory with the default settings:

```bash
cd modules/015-admission-policy-engine/e2e/cpu-starvation/tests/webhook-noisy-neighbour
task run
```

Run the same scenario with a 15-minute hold:

```bash
task run -- --set hold=15m
```

Run every scenario from the suite root:

```bash
cd modules/015-admission-policy-engine/e2e/cpu-starvation
task run
```

Plan about 15 minutes per scenario with the default settings.

## Reports and debugging

Each run leaves its artifacts in the scenario directory; `reports/` is listed in `.gitignore`.
The state directory holds no credentials.
The artifacts are the following files:

- `reports/chainsaw-report.xml` is the JUnit report;
- `reports/state-<NAMESPACE>/summary.txt` is the summary printed by the `analyse` step;
- `reports/state-<NAMESPACE>/probes.log` holds
  `<epoch> <probe> <result> <latency_ms|NA> [webhook=<NAME>] [reason]` lines;
- `reports/state-<NAMESPACE>/probe-infra-error.txt`, `probe-unreachable-error.txt` and `probe-foreign-webhook-error.txt`
  hold the full output of the first probe of each class, if any;
- `reports/state-<NAMESPACE>/watch-*.log` hold `<epoch> <event type>` lines of every watch,
  and `watch-*.exits` hold the exit code of every watch connection;
- `reports/state-<NAMESPACE>/vwc.log` holds the ValidatingWebhookConfiguration polls;
- `reports/state-<NAMESPACE>/phases` holds the epoch of every phase mark: `baseline`, `baseline-end`, `ramp-up`,
  `pressure`, `release`, `recovered` and `end`;
- `reports/state-<NAMESPACE>/kubeconfig-path` holds the path of the kubeconfig copy while the observers run.

On failure, the tests print the module pods, the events of `d8-admission-policy-engine`, the stress pods
and the tail of the probe log.

## Cleanup after an interrupted run

An interrupted run can leave the following changes in the cluster and on the machine that ran it:

- the test namespace `chainsaw-<NAME>` with its pod policy labels and the stress Deployments;
- the VPA of the target with `updateMode: Off`;
- the target pod with a raised CPU request and, in the CPU limit scenario, a CPU limit;
- the kubeconfig copy in `$TMPDIR/ape-cpu-starvation.XXXXXX`.

The reaper removes the stress Deployments and restores the VPA at the deadline;
`task cleanup` removes everything at once.
Do not run it while a scenario runs: it stops the observers of that run.
Run it from the suite root:

```bash
cd modules/015-admission-policy-engine/e2e/cpu-starvation
task cleanup
```

The task cleans up every run recorded in `tests/*/reports/state-*` that is not marked as cleaned.
Then it looks for leftovers by labels and annotations, so it also works when the state directory is gone:
stress Deployments with the `e2e-test=ape-cpu-starvation` label,
VPA objects with the `ape-cpu-starvation.e2e.deckhouse.io/update-mode` annotation,
and module pods with the `ape-cpu-starvation.e2e.deckhouse.io/resized` annotation.
At the end it prints the remaining stress pods; the expected output is `No resources found`.

To clean up by hand, start with the leftovers of the run:

```bash
kubectl get namespaces | grep '^chainsaw-'
kubectl get deployments -A -l e2e-test=ape-cpu-starvation
kubectl -n d8-admission-policy-engine get vpa -o custom-columns='NAME:.metadata.name,MODE:.spec.updatePolicy.updateMode'
kubectl -n d8-admission-policy-engine get pods -o wide
```

Then undo every change in the following order:

1. Delete the test namespace; this also deletes the stress Deployments:

   ```bash
   kubectl delete namespace <NAMESPACE>
   ```

1. Restore the update mode of the VPA.
   The original mode is in the `ape-cpu-starvation.e2e.deckhouse.io/update-mode` annotation;
   `-` means that the field was not set.

   ```bash
   kubectl -n d8-admission-policy-engine patch vpa <VPA_NAME> --type=merge \
     -p '{"spec":{"updatePolicy":{"updateMode":"<MODE>"}}}'
   kubectl -n d8-admission-policy-engine annotate vpa <VPA_NAME> ape-cpu-starvation.e2e.deckhouse.io/update-mode-
   ```

1. Replace the resized pod with a pod built from the Deployment template:

   ```bash
   kubectl -n d8-admission-policy-engine rollout restart deployment/gatekeeper-controller-manager
   kubectl -n d8-admission-policy-engine rollout status deployment/gatekeeper-controller-manager
   ```

1. Delete the kubeconfig copy; its path is in `reports/state-<NAMESPACE>/kubeconfig-path`:

   ```bash
   rm -rf "$(dirname "$(cat reports/state-<NAMESPACE>/kubeconfig-path)")"
   ```

Here `<NAMESPACE>` is the Chainsaw namespace of the run, `<VPA_NAME>` is the VPA of the target Deployment,
and `<MODE>` is the original update mode.
After the cleanup, `kubectl -n d8-admission-policy-engine get pods` shows Ready module pods,
and `kubectl get deployments -A -l e2e-test=ape-cpu-starvation` finds nothing.
