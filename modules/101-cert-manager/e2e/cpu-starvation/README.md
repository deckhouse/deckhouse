# cert-manager CPU starvation e2e tests

The suite checks that the `cert-manager` module survives a node that is short of CPU.
The tests use [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) and only `kubectl` against the current context;
they do not need ssh access to the nodes.

## Overview

Each scenario puts one component of the module under CPU pressure and asserts the following:

- no container of the module restarts, no pod is replaced, and no lease in `d8-cert-manager`
  changes its holder or its `leaseTransitions`;
- the module keeps issuing Certificates under the pressure, slower if necessary, within a budget;
- the module does not flood the API server: the rate of `MODIFIED` watch events of the module leases
  and of the test Certificates does not grow under the pressure and after it;
- the module issues Certificates again and all its Deployments are Ready within `recoverySeconds` after the pressure ends.

The pressure comes from a stress Deployment pinned to the node of the target pod.
The stress pods are Burstable: each has a CPU request, no CPU limit and runs busy loops (`BURN_THREADS`).
Together they request all free allocatable CPU of the node minus 100m.
The suite uses two pressure modes:

- *Noisy neighbour.* One stress pod runs one busy loop per node CPU (`threadsPerPod: 0`).
  The module keeps its own small request and gets only a tiny share of the CPU by weight.
- *High load average.* The test first raises the CPU request of the controller container in place
  to its peak usage (`kubectl patch pod --subresource resize`, no restart).
  The test then starts many small stress pods: 60 busy loops per node CPU (`threadsFactor: 60`),
  4 loops per pod (`threadsPerPod: 4`), at most 100 pods (`maxStressPods`) and no more than the free pod slots of the node.
  The pods split the free CPU of the node equally between them.
  The controller gets the CPU it requests, but competes with many runnable cgroups and waits on a crowded run queue;
  the load average reaches several hundred.
  A variant also sets a CPU limit equal to the request, so CFS quota throttling adds to the pressure.

The high load average needs separate pods.
CFS (Completely Fair Scheduler) shares the CPU between cgroups by their weight,
so busy loops inside one pod compete only with each other:
they raise the load average, but neither take a larger share from other pods nor noticeably delay the controller.
Each pod is a cgroup of its own, so every additional pod adds a runnable competitor of the controller.

## Scenarios

The suite contains four scenarios, each in its own directory under `tests/`.

| Task command | Target | Pressure | Budget of the burst |
| --- | --- | --- | --- |
| `task controller-noisy-neighbour:run` | controller leader (`app=cert-manager`) | noisy neighbour | 15 min |
| `task controller-high-load-average:run` | controller leader | high load average | 10 min |
| `task controller-high-load-average-cpu-limit:run` | controller leader | high load average and a CPU limit equal to the request | 10 min |
| `task webhook-master-noisy-neighbour:run` | webhook (`app=webhook`) on a master | noisy neighbour and a dry-run admission loop | 15 min |

Details of each scenario (steps and pass criteria) are in `tests/<SCENARIO>/README.md`.

## Prerequisites

Install the following tools on the machine that runs the tests:

| Tool | Purpose |
| --- | --- |
| `kubectl` | Cluster access; the current context selects the cluster |
| [Chainsaw](https://kyverno.github.io/chainsaw/) | `chainsaw test`, `chainsaw lint` |
| [Task](https://taskfile.dev/) | Wrapper commands in `Taskfile.yml` |

Installation of Chainsaw and Task is described in `modules/400-descheduler/e2e/descheduler/README.md`.
The helper scripts need a POSIX shell, `awk`, `sed` and `grep`.

The cluster must meet the following requirements:

- the `cert-manager` module is enabled, the `cert-manager` and `webhook` Deployments in `d8-cert-manager` are Ready;
- the user of the context may create, label and delete namespaces, create and delete Deployments,
  Certificates and ClusterIssuers, list pods and nodes cluster-wide
  and patch pods in `d8-cert-manager` (the `resize` subresource);
- the stress image (binding `stressImage`) can be pulled on the nodes;
- the high-load-average scenarios need Kubernetes 1.33 or newer for the in-place pod resize;
- the high-load-average scenarios read the usage of the controller from the metrics API (`kubectl top`);
  without it, the raised request equals `laRequestCpu`.

The reference values were measured with the `vertical-pod-autoscaler` module disabled.
With VPA enabled, the module requests come from VPA, and VPA may recreate a pod during the test,
which the restart check reports as a failure.

**The scenarios deliberately starve a worker node and a master of CPU for several minutes.**
Other workloads on these nodes slow down too. Do not run the suite on a production cluster.

## Bindings

The duration of the phases and the size of the load are Chainsaw bindings with short defaults.
Pass a different value with `--set` after `--` of a task, for example `task run -- --set holdSeconds=900`.

| Binding | Default | Purpose |
| --- | --- | --- |
| `holdSeconds` | `300` | Minimal duration of the pressure and of the baseline window |
| `afterSeconds` | `300` | Duration of the window after the pressure that is checked for a write storm |
| `recoverySeconds` | `120` | Time the module has to issue Certificates and become Ready after the release |
| `threadsFactor` | `1` (noisy neighbour), `60` (high load average) | Busy loops per node CPU in all stress pods together |
| `threadsPerPod` | `0` (noisy neighbour), `4` (high load average) | Busy loops per stress pod; `0` runs all loops in one pod |
| `maxStressPods` | `100` | Largest number of stress pods; the free pod slots of the node (`allocatable.pods` minus running pods minus 5) also limit it |
| `certificateCount` | `30` | Certificates in the baseline burst and in the burst under pressure |
| `burstBudgetSeconds` | `900` (noisy neighbour), `600` (high load average) | Time from the burst until all its Certificates must be Ready |
| `stressImage` | `mirror.gcr.io/library/busybox:1.37.0` pinned by digest | Image of the stress pods; any image with `sh` and `seq` works |
| `laRequestCpu` | `150m` | Minimal CPU request of the controller in the high-load-average scenarios |
| `maxAdmissionFailurePercent` | `1` | Share of failed dry-run admission requests allowed per window (webhook scenario) |

With the defaults, a pressure scenario takes about 20 minutes.
For a manual investigation, use `holdSeconds=900`.

## How the checks work

The pressure scenarios share the same phases, implemented as step templates in `tests/common/step-templates/`:

1. `preflight`: labels the test namespace with `e2e-test=cert-manager-cpu-starvation`,
   checks that the module is Ready, creates the ClusterIssuer `e2e-cpu-starvation-selfsigned`,
   selects the target pod and records restart counts and leases.
   For the controller the target is the pod that holds the lease `cert-manager-controller`.
   When no running controller pod holds the lease, the step fails with the holder and the running pods.
2. `baseline`: starts the watchers, issues `certificateCount` Certificates without pressure,
   waits until `holdSeconds` have passed and marks the end of the baseline.
3. `pressure`: marks the start of the pressure, computes the stress request,
   starts the stress Deployment on the target node and waits until all stress pods run.
   It then issues the same number of Certificates and holds the pressure for `holdSeconds`
   since all stress pods run and until the burst is Ready.
   The high-load-average scenarios mark the start of the pressure earlier, before they resize the controller.
4. `release`: deletes the stress Deployment.
5. `recovery`: issues three Certificates and waits until they and all Deployments of the module are Ready.
   Both waits share one deadline: the release plus `recoverySeconds`.
6. `after-window`: waits `afterSeconds` after the release and compares the write rates.
7. `final-checks`: compares restart counts, pods and leases with the preflight snapshot.

The stress pods together request the allocatable CPU of the node minus the sum of the effective CPU requests of its pods
minus 100m. The effective request of a pod follows the scheduler.
Native sidecars (init containers with `restartPolicy: Always`) add to the sum of the containers,
a regular init container counts together with the sidecars declared before it,
the larger of the two values is taken, and the pod overhead (`spec.overhead`) is added.

The health check covers every Deployment in `d8-cert-manager`: `cert-manager`, `webhook`
and, when they are deployed, `cainjector` and `yandex-dns-webhook`.

The write-storm check runs `kubectl get -w --watch-only --output-watch-events` in the background
for the leases in `d8-cert-manager` and for the Certificates in the test namespace.
It counts `MODIFIED` events per minute in three windows:

- baseline: from the start of the baseline until the mark at its end;
- pressure: from the mark before the resize or the stress Deployment until the stress pods are gone;
- after: from the release until the end of `afterSeconds`.

The start-up of the stress pods and the resize of the controller belong to the pressure window,
so they never raise the baseline rate.
The admission check of the webhook scenario uses the same windows.
A window fails when its rate is more than twice the baseline rate and more than the baseline rate plus 10 per minute,
that is, a window passes when either threshold holds.
Both the baseline and the pressure window contain one burst of the same size, so their rates compare.
The check fails without comparing the rates when the baseline shows that a watch did not work:
the leases have no `MODIFIED` event (the leader renews its lease every 5 seconds),
or the Certificates have fewer `MODIFIED` events than `certificateCount`.
On a test stand, the write rate of the module fell under pressure and returned to the baseline afterwards,
and the lease update rate stayed flat; a retry spiral shows up as growth.

## Running the tests

Validate the test files and the configuration against the Chainsaw schema from the suite directory:

```bash
cd modules/101-cert-manager/e2e/cpu-starvation
task lint
```

The command does not access a cluster.
Do not use `task dry-run` for validation: with `--no-cluster`, the helper scripts still run `kubectl`
against the current context.

Run a single scenario from the suite directory or from the scenario directory:

```bash
cd modules/101-cert-manager/e2e/cpu-starvation
task controller-noisy-neighbour:run

cd tests/controller-noisy-neighbour
task run -- --set holdSeconds=900 --set certificateCount=50
```

Run all scenarios one after another:

```bash
cd modules/101-cert-manager/e2e/cpu-starvation
task run
```

The same tasks are available from the module directory with the `e2e:cpu-starvation:` prefix,
and `e2e:run`, `e2e:lint`, `e2e:dry-run` and `e2e:cleanup` cover all e2e suites of the module:

```bash
cd modules/101-cert-manager
task --list
task e2e:cpu-starvation:controller-noisy-neighbour:run
```

Chainsaw uses the current `kubectl` context. Check it before running:

```bash
kubectl config current-context
kubectl -n d8-cert-manager get deployments
```

## Reports and debugging

Chainsaw writes `JUnit` reports to `tests/<SCENARIO>/reports/` (listed in `.gitignore`).
The helper script keeps the state of a run in `tests/<SCENARIO>/reports/state-<TEST_NAMESPACE>/`,
a directory readable only by the user.
The cleanup of the test prints the summary of the run and removes the directory.
To keep it, set `CPU_STARVATION_KEEP_STATE=true`.

Every API request of the helper script has a deadline of 60 s (`CPU_STARVATION_REQUEST_TIMEOUT`),
and idempotent requests are retried up to 5 times (`CPU_STARVATION_RETRIES`).
Each failed attempt is printed to the step log with its error,
and the script timeouts of the steps exceed the worst case of these retries,
so a slow API server shows up as a logged error instead of a killed step.
The directory holds the following files:

- `marks`: start time of each phase and the number of watch events seen until then;
  the phases are `baseline`, `baseline-end`, `pressure`, `stress-ready`, `release` and `end`;
- `events-leases.log`, `events-certificates.log`: the raw watch events;
- `events-leases.err`, `events-certificates.err`: the errors of the watches;
- `writes-summary`, `admission-summary`: the tables the checks print;
- `snapshot-before`, `snapshot-after`: restart counts and leases;
- `admission.log`: one line per dry-run admission request of the webhook scenario;
- `kubeconfig-path`: the path of the kubeconfig copy for the background processes;
- `bg-<NAME>.pid`, `bg-deadline`: the background processes and the time they stop at the latest.

### Background processes

The watchers and the admission loop run in the background between the steps.
Chainsaw deletes the temporary kubeconfig of a step when the step script ends,
so the helper script copies it (`kubectl config view --raw --minify --flatten`)
and starts the background processes with the copy.
The copy holds credentials, so it is kept outside `reports/`,
in a directory that `mktemp -d` creates as `${TMPDIR:-/tmp}/cert-manager-cpu-starvation.XXXXXX` with mode `0700`.
The copy is removed when the background processes are stopped.
An error that shows the request never reached the API server (no kubeconfig, no connection,
failed authentication or authorization) is an infrastructure failure.
The test fails with a message that says so, and the error is never counted as a failure of the module.

A background process stops when its pid file disappears or at the deadline in `bg-deadline`.
The deadline is the longest run of the phases plus one hour,
so an interrupted run does not leave the processes running forever.

### Cleanup after an interrupted run

The cleanup of every scenario stops the background processes, removes the kubeconfig copy,
deletes the stress Deployment, the test namespace and the ClusterIssuer,
recreates a resized controller pod from its Deployment template and removes the state directory.
An interrupted run, for example with `Ctrl+C`, may skip the cleanup.
The background processes ignore `SIGINT`, so they keep running until their deadline.

Remove the leftovers from the suite directory, using the same kubectl context as the run:

```bash
task cleanup
```

The task performs the following actions:

- stops the background processes of every run whose state directory is in `tests/*/reports/`;
- deletes the stress Deployments with the label `e2e-test=cert-manager-cpu-starvation`;
- deletes the `chainsaw-*` namespaces with the label `e2e-test=cert-manager-cpu-starvation`,
  through their Project when multitenancy-manager made one;
- deletes the ClusterIssuer `e2e-cpu-starvation-selfsigned`;
- restarts the controller Deployment when a run resized the controller pod;
- removes the state directories and the kubeconfig copies.

The task deletes only objects with the label of the suite.
Other e2e suites also run in `chainsaw-*` namespaces and may label their stress Deployments with `app=cpu-stress`;
the task leaves them alone.
Do not run `task cleanup` while a scenario of this suite runs: it deletes the namespace of the running test too.
To remove the leftovers manually, run the following commands from the suite directory, replacing `<TEST_NAMESPACE>`
with the `chainsaw-*` namespace of the interrupted run:

```bash
pkill -f 'cpu-starvation/tests/common/scripts/watch-writes.sh'
pkill -f 'cpu-starvation/tests/common/scripts/e2e.sh admission-loop'
kubectl delete deployments -A -l e2e-test=cert-manager-cpu-starvation
kubectl delete projects.deckhouse.io <TEST_NAMESPACE> --ignore-not-found
kubectl delete namespace <TEST_NAMESPACE> --ignore-not-found
kubectl delete clusterissuer e2e-cpu-starvation-selfsigned --ignore-not-found
kubectl -n d8-cert-manager rollout restart deployment/cert-manager
rm -rf tests/*/reports/state-* "${TMPDIR:-/tmp}"/cert-manager-cpu-starvation.*
```

The `rollout restart` is needed only after a high-load-average scenario,
which resizes the controller pod in place.

## Reference values

The values below were measured on a stand with one master (8 CPU), two workers (6 CPU) and VPA disabled.
The bursts in the table had 50 Certificates (`certificateCount=50`), not the default 30,
so compare them with a run that sets the same value.
The high-load-average row was measured with one stress pod, before the scenarios moved to separate stress pods;
it is not a reference for the current scenarios yet.

| Case | Result |
| --- | --- |
| Baseline, 50 Certificates | Ready in 68 s |
| Noisy neighbour on the controller node, 50 Certificates | Ready in 520 s, no restarts, no lease change |
| High load average (request 150m, load average about 480), 50 Certificates, one stress pod with 60 busy loops per CPU | Ready in 297 s |
| Controller limited to 10m CPU, 50 Certificates | 3 Ready in 15 min; the queue drained 45 s after the limit was lifted |
| Noisy neighbour on the master with the webhook (4 of 8 CPU requested, 8 busy loops) | No admission failures, no restarts |
