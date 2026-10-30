# Dex throttled on a node with a high load average

## Summary

This scenario is the [high load average scenario](../dex-high-load-average/README.md) with CPU limits.
The limits of `dex` and `kube-rbac-proxy` are set in place equal to their raised requests, 500m and 50m by default.
Under the load Dex hits its CFS quota and is throttled, while it also competes with the burner pods for the CPU.

The module itself sets no CPU limit on Dex. A limit comes from a manual change or from a policy of the cluster,
and this scenario checks that such a limit does not turn slowness into restarts.

## Prerequisites

The scenario needs the cluster and the tools described in the [suite README](../../README.md), and Kubernetes 1.33 or newer.

## Test steps

The steps are the steps of the high load average scenario.
The step `raise-dex-request` also sets the CPU limits and waits for the kubelet to apply them.

## Pass and fail criteria

The scenario has the checks of the suite and one more:
at least 90% of the logins under the pressure succeed (`minPressureSuccess`).
In a reference run with the fix, all 790 logins under the pressure succeeded with a p95 login time of 1.7 seconds.

## Running

To run the scenario, run the following commands:

```bash
cd modules/150-user-authn/e2e/cpu-starvation/tests/dex-high-load-average-cpu-limit
task run
```

## Cleanup

Limits cannot be removed in place.
The cleanup restores the VPA update mode and deletes the resized Dex pod, so the Deployment recreates it without the limits.
