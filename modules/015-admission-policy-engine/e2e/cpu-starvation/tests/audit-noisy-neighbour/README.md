# Audit under a noisy neighbour

This scenario checks that the Gatekeeper audit keeps running and does not flood the API server
while a neighbour takes all free CPU of its node.

## Summary

The target is the `manager` container of `gatekeeper-audit`.
Without system nodes the audit pod can run on any node, including a master, so the stress pod tolerates every taint.

The stress pod is Burstable: it requests the free CPU of the node, has no CPU limit
and runs one burner thread per node CPU.
The audit runs every 60 seconds and lists every matched object from the API server,
so a starved audit cycle can take longer than its interval.
The scenario asserts that the audit completes a cycle after the release and that its status writes do not grow.

The admission probes keep running too: the webhook usually runs on another node and must not be affected.

## Test steps

The scenario runs the same steps as `webhook-noisy-neighbour` with `TARGET=audit`:

| Step | Description |
| ---- | ----------- |
| `preflight` | Labels the test namespace, checks the module, finds the audit pod |
| `freeze-vpa` | Sets `updateMode: Off` on the VPA of `gatekeeper-audit`, so the VPA updater does not evict it; the cleanup restores it |
| `start-observers` | Takes snapshots and starts the probe loop, the watches and the webhook configuration poll |
| `baseline` | Collects the baseline for `baselineWindow` (3m) |
| `start-pressure` | Computes the free CPU of the audit node and starts the stress Deployment |
| `hold-pressure` | Keeps the pressure for `hold` (5m) |
| `release-and-recover` | Deletes the stress Deployment and waits up to `recovery` (120s) for Ready pods and passing probes |
| `observe-after-release` | Observes for `afterWindow` (5m) and waits up to `auditRecovery` (5m) for an audit cycle that ends after the release |
| `analyse` | Asserts the results and prints the summary |

## Pass and fail criteria

The scenario passes when all of the following holds:

- an audit cycle ends after the release within `auditRecovery`;
- the MODIFIED rate of `constraintpodstatuses` and `constrainttemplatepodstatuses` does not exceed both
  2x the baseline and baseline + 10 per minute;
- no container of `gatekeeper-audit` restarts, including `constraint-exporter`, which has no probes;
- the admission probes stay within the 1% failure budget, and `d8-admission-policy-engine-config` never disappears.

The audit check reads `gatekeeper_audit_last_run_end_time` through the API server pod proxy
and falls back to the newest `status.auditTimestamp` of the constraints.
The summary also shows whether an audit cycle completed during the pressure; this value is informational.

## Running

Run the scenario from this directory:

```bash
task run
```

Run it with a longer pressure for a manual check:

```bash
task run -- --set hold=15m
```

## Troubleshooting

Read the audit metrics directly to see the duration of the last cycles:

```bash
POD=$(kubectl -n d8-admission-policy-engine get pod -l control-plane=audit-controller -o name | head -n 1)
kubectl get --raw "/api/v1/namespaces/d8-admission-policy-engine/pods/${POD#pod/}:8888/proxy/metrics" |
  grep '^gatekeeper_audit'
```
