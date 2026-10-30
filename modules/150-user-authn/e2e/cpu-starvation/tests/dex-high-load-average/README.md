# Dex on a node with a high load average

## Summary

This scenario checks Dex on a node with a load average of several hundred.
Dex first gets its CPU request raised in place to its usage under the load, 500m by default.
A burner then starts up to `maxBurnerPods` (100) pods of `threadsPerPod` (3) busy threads, 60 threads per CPU in total.
Together the pods request the rest of the node CPU.

The CPU scheduler shares the CPU between cgroups by their weight, and every burner pod is its own cgroup.
By weight, Dex still gets the CPU it requests, but it competes with every burner pod for each slice of it.
A single pod with the same threads would raise only the load average:
its threads would compete with each other and not with Dex.

On the reference stand the load through the ingress had no failed logins and no restarts,
the median login time grew from 1.4 to 14 seconds, and the recovery was immediate.
This scenario sends the load to the Dex pod directly, without ingress-nginx, which was the bottleneck on that stand.
In a reference run of this scenario with the fix, all 923 logins under the pressure succeeded,
and the p95 login time grew from 1.3 to 1.8 seconds.
The threshold `minPressureSuccess` of `0.9` leaves a margin for other clusters.

## Prerequisites

The scenario needs the cluster and the tools described in the [suite README](../../README.md), and Kubernetes 1.33 or newer.

## Test steps

The steps are the steps of the [noisy neighbour scenario](../dex-noisy-neighbour/README.md) with the following differences:

- `assert-module-ready` also checks that the API server runs Kubernetes 1.33 or newer.
- The step `raise-dex-request` runs after `deploy-load`.
  It saves the update mode of the VPA `dex` in an annotation and sets `updateMode: Off`,
  labels the Dex pod, resizes `dex` to `dexRequest` and `kube-rbac-proxy` to `proxyRequest` in place,
  and waits for the kubelet to apply them.
- In `apply-pressure`, the burner is `threadsFactor` (60) threads per CPU in pods of `threadsPerPod` (3) threads.
  The number of pods is limited by `maxBurnerPods` (100) and by the free pod slots of the node minus 5.
  The pods together request the CPU of the node left after the raised request.
- `hold-pressure` also checks every 30 seconds that the VPA `dex` keeps `updateMode: Off`
  and that the kubelet reports the raised CPU request of `dex` in the status of the Dex pod.
  The step fails when either changes, for example, with `dex CPU request changed from 500m to 20m`.
  A failed read of the API server is logged and retried at the next check.
  The VPA check runs only when the run switched the VPA to `Off`.

## Pass and fail criteria

The scenario has the checks of the suite and one more:
at least 90% of the logins under the pressure succeed (`minPressureSuccess`).

## Running

To run the scenario, run the following commands:

```bash
cd modules/150-user-authn/e2e/cpu-starvation/tests/dex-high-load-average
task run
```

## Cleanup

The cleanup of `raise-dex-request` restores the VPA update mode saved in the annotation,
deletes the resized Dex pod, and waits for the `dex` Deployment to roll out.
The Deployment recreates the pod from its template with the module resources.
