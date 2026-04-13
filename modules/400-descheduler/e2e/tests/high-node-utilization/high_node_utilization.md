# Descheduler HighNodeUtilization

## What it does

This [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) end-to-end test validates that the Kubernetes **Descheduler** correctly consolidates workloads using the **HighNodeUtilization** plugin. The plugin evicts pods from **underutilized** nodes so the scheduler can pack them onto busier nodes — freeing up entire nodes for cost savings or cluster autoscaler downscaling.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-descheduler-ready` | Asserts descheduler deployment exists and has ready replicas |
| 2 | `check-minimum-nodes` | Verifies cluster has at least 2 nodes |
| 3 | `backup-descheduler-policy` | Backs up current policy ConfigMap (cleanup restores it) |
| 4 | `apply-descheduler-policy` | Applies HighNodeUtilization policy from `files/descheduler-policy.yaml` |
| 5 | `restart-descheduler` | Restarts descheduler and asserts it becomes ready |
| 6 | `create-spread-workload` | Pins 1 pause pod per node via `nodeName` — makes all nodes underutilized |
| 7 | `assert-pods-running` | Asserts representative spread pods are in Running phase |
| 8 | `wait-for-descheduler-cycle` | Polls descheduler logs for HighNodeUtilization execution |
| 9 | `verify-pod-consolidation` | Checks pod distribution and eviction events |

**Cleanup:** `cleanup` block on step 3 restores the original descheduler policy. Test namespace pods are auto-deleted by Chainsaw.

## Key Configuration

- **Thresholds:** cpu=20%, memory=20%, pods=20% (nodes below ALL = underutilized)
- **DefaultEvictor:** `nodeFit: true` (only evict if pod fits elsewhere)
- **Excluded namespaces:** `kube-system`

## Prerequisites

- Multi-node cluster (minimum 2 nodes)
- Descheduler pre-installed in `d8-descheduler` namespace
- Recommended: kube-scheduler with `MostAllocated` scoring strategy

## How to Run

```bash
# From the e2e directory
task run:high-node-utilization

# Or directly
chainsaw test --test-dir ./tests/high-node-utilization/
```
