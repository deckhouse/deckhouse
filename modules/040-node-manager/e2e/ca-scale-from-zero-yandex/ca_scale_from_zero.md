# CA Scale from Zero Test (Yandex Cloud / MCM)

## Description

Tests basic Cluster Autoscaler scale-from-zero functionality with Priority
Expander on Yandex Cloud (MCM mode). Both NodeGroups use working InstanceClasses
with `minPerZone: 0`. CA must discover groups with zero nodes across all
availability zones and scale up the higher-priority group.

## What is tested

| Step | Check |
|------|-------|
| 1 | CA discovers both NodeGroups with 0 replicas across all zones |
| 2 | Priority Expander selects e2e-worker-100 (highest priority) |
| 3 | CA scales e2e-worker-100 from 0 to required size (distributed across zones) |
| 4 | Nodes become Ready, pods are scheduled on e2e-worker-100 nodes |
| 5 | e2e-worker-50 remains at 0 nodes (not selected) |

## Prerequisites

- Kubernetes cluster with Yandex Cloud provider (MCM mode)
- Cluster Autoscaler deployed in `d8-cloud-instance-manager`
- Working `YandexInstanceClass` named `worker-small`

## Run

```bash
chainsaw test --test-dir ./ca-scale-from-zero-yandex/
```

## Estimated Duration

~15 minutes (node provisioning across zones + pod scheduling)

## Resources created by the test

- `NodeGroup/e2e-worker-100` — priority 100, references working IC `worker-small`
- `NodeGroup/e2e-worker-50` — priority 50, references working IC `worker-small`
- `Deployment/e2e-nginx` — 3 replicas with nodeSelector, tolerations, podAntiAffinity

All resources are cleaned up in the `finally` block.
