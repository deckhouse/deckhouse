# Descheduler LowNodeUtilization

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates the Kubernetes **Descheduler LowNodeUtilization** plugin correctly rebalances pods across cluster nodes.

**What it does:** Creates an artificial pod imbalance (10 pods pinned to one node), then verifies the descheduler evicts pods from the overloaded node so the scheduler can redistribute them.

## Prerequisites

- Multi-node Kubernetes cluster (minimum 2 nodes)
- Descheduler pre-installed in the `d8-descheduler` namespace
- Chainsaw CLI installed. Use ../E2e.md for instructions.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `check-prerequisites` | Verifies cluster has 2+ nodes and descheduler deployment exists |
| 2 | `configure-descheduler` | Backs up existing policy, applies LowNodeUtilization config (thresholds: 20%, targetThresholds: 50%), restarts descheduler |
| 3 | `create-imbalanced-workload` | Creates 10 `pause` pods pinned to one node via `nodeName` |
| 4 | `verify-descheduler-results` | Polls descheduler logs (up to 5 min) for `LowNodeUtilization` execution, checks pod distribution and eviction events |

**Cleanup:** `finally` block in step 4 restores the original descheduler policy from backup.

## Policy Config

- **Underutilized** (ALL below): cpu < 20%, memory < 20%, pods < 20%
- **Overutilized** (ANY above): cpu > 50%, memory > 50%, pods > 50%
- **DefaultEvictor**: `nodeFit: true` — only evict pods that can be scheduled elsewhere

## Running

```bash
chainsaw test --test-dir ./low-node-utilization/
```

## Pass/Fail Criteria

- **Pass:** Descheduler logs show `LowNodeUtilization` execution; pods get evicted and redistributed
- **Fail:** Fewer than 2 nodes, descheduler not found, or plugin doesn't execute within 5 minutes
