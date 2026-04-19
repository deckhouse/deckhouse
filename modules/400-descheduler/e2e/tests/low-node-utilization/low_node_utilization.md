# Descheduler LowNodeUtilization

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates the Kubernetes **Descheduler LowNodeUtilization** plugin correctly rebalances pods across cluster nodes.

**What it does:** Creates an artificial pod imbalance (10 pods pinned to one node), then verifies the descheduler evicts pods from the overloaded node so the scheduler can redistribute them.

## Prerequisites

- Multi-node Kubernetes cluster (minimum 3(include master-node) nodes)
- Descheduler pre-installed in the `d8-descheduler` namespace
- Chainsaw CLI installed. See `../../E2E.md` for instructions.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-descheduler-ready` | Asserts descheduler deployment exists and has ready replicas |
| 2 | `check-minimum-nodes` | Verifies cluster has at least 2 nodes |
| 3 | `backup-descheduler-policy` | Backs up current policy ConfigMap (cleanup restores it) |
| 4 | `apply-descheduler-policy` | Applies LowNodeUtilization policy from `files/descheduler-policy.yaml` |
| 5 | `restart-descheduler` | Restarts descheduler and asserts it becomes ready |
| 6 | `create-imbalanced-workload` | Creates 10 pause pods pinned to one node via `nodeName` |
| 7 | `assert-pods-running` | Asserts representative pods are in Running phase |
| 8 | `wait-for-descheduler-cycle` | Polls descheduler logs for LowNodeUtilization execution |
| 9 | `verify-pod-redistribution` | Checks pod distribution across nodes |

**Cleanup:** `cleanup` block on step 3 restores the original descheduler policy. Test namespace pods are auto-deleted by Chainsaw.

## Policy Config

- **Underutilized** (ALL below): cpu < 20%, memory < 20%, pods < 20%
- **Overutilized** (ANY above): cpu > 50%, memory > 50%, pods > 50%
- **DefaultEvictor**: `nodeFit: true` — only evict pods that can be scheduled elsewhere

## Running

```bash
# From the e2e directory
task run:low-node-utilization

# Or directly
chainsaw test --test-dir ./tests/low-node-utilization/
```

## Pass/Fail Criteria

- **Pass:** Descheduler logs show `LowNodeUtilization` execution; pods get evicted and redistributed
- **Fail:** Fewer than 3 nodes, descheduler not found, or plugin doesn't execute within 10 minutes, if `deschedulingInterval: Frequent` is used.
