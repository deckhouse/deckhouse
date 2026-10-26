# Webhook under a noisy neighbour

This scenario checks that the Gatekeeper webhook keeps admitting and denying Pods
while a neighbour takes all free CPU of its node.

## Summary

The target is the `manager` container of `gatekeeper-controller-manager`.
The pod usually runs on a master or system node, so the stress pod tolerates every taint.

The stress pod is Burstable: it requests the free CPU of the node
(allocatable minus the requests of all pods minus 100m), has no CPU limit and runs one burner thread per node CPU.
The webhook keeps its own request of about 100m, so under contention it gets a small share of the CPU by weight.
With several webhook replicas, every node that runs a replica gets its own stress pod.

The scenario passed on a cluster with one master and one webhook replica;
the suite README lists the numbers in [Reference results](../../README.md#reference-results).

## Test steps

The scenario runs the following steps:

| Step | Description |
| ---- | ----------- |
| `preflight` | Labels the test namespace for the Baseline standard with the deny action, checks the module, finds the webhook pods |
| `freeze-vpa` | Sets `updateMode: Off` on the VPA of `gatekeeper-controller-manager`; the cleanup restores it |
| `start-observers` | Takes snapshots and starts the probe loop, the watches and the webhook configuration poll |
| `baseline` | Collects the baseline for `baselineWindow` (3m) |
| `start-pressure` | Computes the free CPU of the node and starts the stress Deployment |
| `hold-pressure` | Keeps the pressure for `hold` (5m) |
| `release-and-recover` | Deletes the stress Deployment and waits up to `recovery` (120s) for Ready pods and passing probes |
| `observe-after-release` | Observes for `afterWindow` (5m) and waits for an audit cycle after the release |
| `analyse` | Asserts the results and prints the summary |

## Pass and fail criteria

The scenario passes when all of the following holds:

- at most 1% of the admission probes fail under pressure and after the recovery,
  and the privileged Pod is never admitted;
- at most 1% of the probes find the API server unreachable under pressure and after the recovery;
- the module recovers within `recovery`;
- the MODIFIED rate of leases and Gatekeeper status objects does not exceed both 2x the baseline
  and baseline + 10 per minute;
- `d8-admission-policy-engine-config` never disappears and keeps its UID and webhooks;
- no container of the module restarts and no module pod is replaced;
- an audit cycle completes within `auditRecovery` after the release.

The suite README describes every check: `../../README.md`.

## Running

Run the scenario from this directory:

```bash
task run
```

Run it with a longer pressure for a manual check:

```bash
task run -- --set hold=15m
```

The summary is saved to `reports/state-<NAMESPACE>/summary.txt`.
After an interrupted run, remove the leftovers with `task cleanup`.

## Troubleshooting

The webhook uses `failurePolicy: Fail` with a 15-second timeout.
Probe errors with `failed calling webhook` or `context deadline exceeded` mean that the webhook did not answer in time.
Check the readiness of the webhook pod and its events:

```bash
kubectl -n d8-admission-policy-engine get events --field-selector reason=Unhealthy
kubectl -n d8-admission-policy-engine get pods -l control-plane=controller-manager -o wide
```
