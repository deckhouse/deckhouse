# CA Priority Fallback Test (DVP / CAPI)

## Description

Tests the Cluster Autoscaler Priority Expander and backoff mechanism when the
highest-priority NodeGroup references a broken InstanceClass.

## What is tested

| Step | Check |
|------|-------|
| 1 | CA discovers both NodeGroups (priority 100 and 50) |
| 2 | Priority Expander selects e2e-worker-100 (highest priority) |
| 3 | CA attempts to scale up e2e-worker-100 — fails (broken `virtualMachineClassName`) |
| 4 | After ~15 min CA enters backoff for e2e-worker-100 |
| 5 | Priority Expander falls back to e2e-worker-50 |
| 6 | Nodes are provisioned in e2e-worker-50, pods are scheduled |

## Prerequisites

- Kubernetes cluster with DVP cloud provider (CAPI mode)
- Cluster Autoscaler deployed in `d8-cloud-instance-manager`
- Working `DVPInstanceClass` named `worker`
- `jq` installed

## Run

```bash
chainsaw test --test-dir ./ca-priority-fallback-dvp/
```

## Estimated Duration

~25 minutes (15 min backoff + ~10 min node provisioning)

## Resources created by the test

- `DVPInstanceClass/e2e-worker-broken` — clone of `worker` with `virtualMachineClassName: DOES-NOT-EXIST`
- `NodeGroup/e2e-worker-100` — priority 100, references broken IC
- `NodeGroup/e2e-worker-50` — priority 50, references working IC `worker`
- `Deployment/e2e-nginx` — 3 replicas with nodeSelector, tolerations, podAntiAffinity

All resources are cleaned up in the `finally` block.
