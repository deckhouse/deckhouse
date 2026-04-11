# Descheduler HighNodeUtilization

## What it does

This [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) end-to-end test validates that the Kubernetes **Descheduler** correctly consolidates workloads using the **HighNodeUtilization** plugin. The plugin evicts pods from **underutilized** nodes so the scheduler can pack them onto busier nodes — freeing up entire nodes for cost savings or cluster autoscaler downscaling.

## Test steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `check-prerequisites` | Verifies cluster has 2+ nodes and descheduler deployment exists in `d8-descheduler` namespace |
| 2 | `configure-descheduler` | Backs up existing policy, applies HighNodeUtilization config (thresholds: cpu/memory/pods < 20%), restarts descheduler |
| 3 | `create-spread-workload` | Pins 1 `pause` pod per node via `nodeName` — makes all nodes underutilized |
| 4 | `verify-descheduler-results` | Polls descheduler logs (up to 5 min) for HighNodeUtilization execution, checks eviction events and pod consolidation |

**Cleanup:** `finally` block in step 4 restores the original descheduler policy from backup. Test namespace pods are auto-deleted by Chainsaw.

## Key configuration

- **Thresholds:** cpu=20%, memory=20%, pods=20% (nodes below ALL = underutilized)
- **DefaultEvictor:** `nodeFit: true` (only evict if pod fits elsewhere)
- **Excluded namespaces:** `kube-system`

## Prerequisites

- Multi-node cluster (minimum 2 nodes)
- Descheduler pre-installed in `d8-descheduler` namespace
- Recommended: kube-scheduler with `MostAllocated` scoring strategy

## How to run

```bash
chainsaw test --test-dir ./high-node-utilization/
```
