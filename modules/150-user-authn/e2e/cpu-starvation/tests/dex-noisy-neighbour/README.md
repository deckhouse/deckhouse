# Dex on a node with a noisy neighbour

## Summary

This scenario checks that Dex keeps running while a neighbour pod takes the free CPU of its node.
Dex keeps its own small request, so under contention it gets a share of the node by that weight.

The scenario checks the fix from the merge request "[user-authn] Keep Dex running on a node short of CPU".
**On code without the fix, the scenario fails at the restart check.**
A run of this scenario on `main` without the fix restarted Dex 5 times and kube-rbac-proxy 4 times,
and the logins under the pressure failed with "connection refused".
With the fix, the scenario passed with no restarts.

The manual reproduction of the incident, with a 15-minute pressure and the load through the ingress, gave other numbers:
Dex restarted 8 times and kube-rbac-proxy 5 times, logins succeeded in 1–3% of the attempts,
and the restart back-off kept them down for more than 4 minutes after the pressure ended.

## Prerequisites

The scenario needs the cluster and the tools described in the [suite README](../../README.md).

## Test steps

The steps run in the following order:

| Step | Name | Description |
| ---- | ---- | ----------- |
| 1 | `assert-module-ready` | Asserts the CRDs, the Available `dex` Deployment and at least 2 Ready nodes |
| 2 | `deploy-load` | Creates the DexClient, the load pod outside the Dex node aimed at the Dex pod IP, two users with a random password; waits for a login and a refresh |
| 3 | `record-baseline` | Records the Dex restarts, the leases and the AuthRequest count, marks `baseline`, runs the load for `baseline` seconds |
| 4 | `apply-pressure` | Starts one burner pod on the Dex node: request = node allocatable − effective pod requests − `cpuMargin`, one thread per CPU; marks `pressure` |
| 5 | `hold-pressure` | Keeps the pressure for `hold` seconds and logs the Dex pod and the AuthRequest count |
| 6 | `release-pressure` | Deletes the burner and marks `release` |
| 7 | `assert-pods-recover` | Every pod of `d8-user-authn` is Ready within `recovery` seconds, none in `CrashLoopBackOff` |
| 8 | `observe-after` | Keeps the load running until `after` seconds after the release and marks `end` |
| 9 | `evaluate` | Login success per phase, recovery window, write storm, Dex restarts, leases |
| 10 | `drain-authrequests` | Stops the load and waits until the AuthRequests return to the count before the run |
| 11 | `verdict` | Fails when any check above failed |

## Pass and fail criteria

The scenario passes when every check of the suite passes.
The login success ratio under the pressure is only reported, because without CPU Dex cannot serve logins in time.
The checks are the following:

- The Dex pod is not replaced, and the restart counts of `dex` and `kube-rbac-proxy` do not grow.
- At least 95% of the logins started 120–180 seconds after the release succeed.
- No write storm: for each watched resource, the rate does not exceed twice the baseline
  and the events per minute the baseline plus 10 at the same time.
  The rate of the leases is the events per minute; the rate of `useraccounts` and `refreshtokens` is the events per completed flow.
- The leases of `d8-user-authn` keep their holder and transition count.
- The AuthRequests return to the count before the run plus 5 within 16 minutes after the load stops.

## Running

To run the scenario, run the following commands:

```bash
cd modules/150-user-authn/e2e/cpu-starvation/tests/dex-noisy-neighbour
task run
```

For a manual run with a longer hold, override the binding:

```bash
task run -- --set hold=900
```

## Notes

On a cluster with the `vertical-pod-autoscaler` module, the scenario does not change the VPA of Dex.
With the fix, the VPA may try to raise the Dex request during the run. The node is fully requested,
so such a resize stays deferred, and the step logs show the Dex resources at the start of the run.

In a cluster with two Dex replicas, the scenario stresses the node of the first replica by name.
The load goes to the IP address of that pod and not to the Service, so the second replica takes no part of it.
