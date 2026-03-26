# CA Priority Fallback Test (Yandex Cloud / MCM)

## Description

Tests the Cluster Autoscaler Priority Expander and backoff mechanism when the
highest-priority NodeGroup references a broken InstanceClass. Yandex Cloud uses
MCM (Machine Controller Manager) with multiple availability zones, so backoff
takes longer (~15 min per zone).

## What is tested

| Step | Check |
|------|-------|
| 1 | CA discovers both NodeGroups (priority 100 and 50) across all zones |
| 2 | Priority Expander selects e2e-worker-100 (highest priority) |
| 3 | CA attempts to scale up e2e-worker-100 — fails (broken `imageID`) |
| 4 | After ~45 min (3 zones × ~15 min) CA enters backoff for all e2e-worker-100 MDs |
| 5 | Priority Expander falls back to e2e-worker-50 |
| 6 | Nodes are provisioned in e2e-worker-50, pods are scheduled |

## Prerequisites

- Kubernetes cluster with Yandex Cloud provider (MCM mode)
- Cluster Autoscaler deployed in `d8-cloud-instance-manager`
- Working `YandexInstanceClass` named `worker-small`
- `jq` installed

## Run

```bash
chainsaw test --test-dir ./ca-priority-fallback-yandex/
```

## Estimated Duration

~55 minutes (45 min backoff across 3 zones + ~10 min node provisioning)

## Resources created by the test

- `YandexInstanceClass/e2e-worker-broken` — clone of `worker-small` with `imageID: fd8INVALID000000000`
- `NodeGroup/e2e-worker-100` — priority 100, references broken IC
- `NodeGroup/e2e-worker-50` — priority 50, references working IC `worker-small`
- `Deployment/e2e-nginx` — 3 replicas with nodeSelector, tolerations, podAntiAffinity

All resources are cleaned up in the `finally` block.
