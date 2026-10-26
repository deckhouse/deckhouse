# Controller under a high load average

The scenario checks that the cert-manager controller keeps working on a node with a load average of several hundred.

## Summary

The test raises the CPU request of the `cert-manager` container of the controller leader in place,
without a restart, to its peak usage in the baseline, but not below `laRequestCpu` (default `150m`).
The test then starts many small stress pods on the node: 60 busy loops per node CPU (`threadsFactor: 60`),
4 loops per pod (`threadsPerPod: 4`), at most `maxStressPods` (default 100) pods within the free pod slots of the node.
Together the pods request the rest of the node.
Each pod is a cgroup of its own: CFS shares the CPU between cgroups by weight,
so only separate pods, not loops inside one pod, crowd the run queue of the controller.
The controller is entitled to the CPU it requests, but waits on a crowded run queue.

Under this pressure, a burst of `certificateCount` Certificates must become Ready within 10 minutes,
the controller must not restart or lose its lease, and its write rate must not grow.

## Prerequisites

The scenario resizes a running pod, which needs Kubernetes 1.33 or newer.
The peak usage comes from the metrics API (`kubectl top`); without it, the request equals `laRequestCpu`.

## Test steps

Most steps come from the step templates in `../common/step-templates/`:

| Step | Description |
| --- | --- |
| `require-in-place-resize` | Checks the Kubernetes version |
| `preflight` | Labels the test namespace, checks the module is Ready, creates the ClusterIssuer, selects the leader pod, records restarts and leases |
| `baseline` | Starts the watchers, issues the baseline burst and samples the controller usage, waits until `holdSeconds` have passed, marks the end of the baseline |
| `raise-cpu-request` | Marks the start of the pressure, so the resize is not part of the baseline, and resizes the CPU request of the controller container in place |
| `pressure` | Starts the stress pods on the leader node, issues the burst, holds the pressure for `holdSeconds` and until the burst is Ready |
| `release` | Deletes the stress Deployment |
| `recovery` | Issues three Certificates and waits until they and all module Deployments are Ready, at most `recoverySeconds` after the release |
| `after-window` | Waits `afterSeconds` and compares the write rates with the baseline |
| `final-checks` | Compares restart counts, pods and leases with the preflight snapshot |

The cleanup recreates the controller pod from its Deployment template, which drops the raised resources,
and waits for the rollout. It also stops the watchers and deletes the stress Deployment,
the test namespace and the ClusterIssuer.
The recreated pod may take over the lease; this happens after the checks and does not affect the result.

## Pass criteria

The scenario passes when all of the following holds:

- all Certificates of the burst under pressure are Ready within `burstBudgetSeconds` (default 600);
- the three recovery Certificates and all Deployments of the module are Ready within `recoverySeconds` (default 120)
  after the release;
- the baseline has `MODIFIED` events of the leases and at least `certificateCount` `MODIFIED` events of the Certificates;
- the rate of `MODIFIED` events of the leases and of the Certificates under pressure and after it
  is at most twice the baseline rate or at most the baseline rate plus 10 per minute;
- no container of `d8-cert-manager` restarted, no pod was replaced, no lease changed its holder or `leaseTransitions`.

On a stand with 6-CPU workers, a request of 150m and `certificateCount=50`, 50 Certificates became Ready in 297 s
at a load average of about 480.
That run used one stress pod with 60 busy loops per CPU, before the scenario moved to separate stress pods.

## Running

Run the scenario from its directory:

```bash
task run
task run -- --set holdSeconds=900 --set laRequestCpu=200m
```
