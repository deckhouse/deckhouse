# Descheduler Exclude Namespaces from Processing

## What it does

This [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test validates the Deckhouse descheduler patch (`001-filter-pods-in-deckhouse-namespaces.patch`) that prevents eviction of pods in `d8-*` and `kube-system` namespaces. The patch adds a constraint to `DefaultEvictor` that rejects pods from protected namespaces.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-descheduler-ready` | Asserts descheduler deployment exists and has ready replicas |
| 2 | `check-minimum-nodes` | Verifies cluster has at least 2 nodes |
| 3 | `backup-descheduler-policy` | Backs up current policy ConfigMap (cleanup restores it) |
| 4 | `apply-descheduler-policy` | Applies LowNodeUtilization policy from `files/descheduler-policy.yaml` |
| 5 | `restart-descheduler` | Restarts descheduler and asserts it becomes ready |
| 6 | `create-protected-namespace` | Creates `d8-chainsaw-test` namespace from `files/protected-namespace.yaml` (cleanup deletes it) |
| 7 | `create-protected-workload` | Creates 5 pods in `d8-chainsaw-test` pinned to one node |
| 8 | `create-regular-workload` | Creates 5 pods in the regular test namespace pinned to the same node |
| 9 | `assert-protected-pods-running` | Asserts representative protected pods are Running |
| 10 | `assert-regular-pods-running` | Asserts representative regular pods are Running |
| 11 | `wait-for-descheduler-cycle` | Polls descheduler logs for LowNodeUtilization execution |
| 12 | `verify-protected-pods-not-evicted` | Asserts all 5 protected pods are still Running |
| 13 | `verify-no-evictions-in-protected-namespace` | Checks zero eviction events in `d8-chainsaw-test` |
| 14 | `verify-namespace-filtering-logs` | Checks descheduler logs for namespace filtering messages |

**Cleanup:** `cleanup` block on step 3 restores the original descheduler policy. `cleanup` block on step 6 deletes `d8-chainsaw-test` namespace. Test namespace pods are auto-deleted by Chainsaw.

## Pass/Fail Criteria

- **Pass:** All 5 protected pods remain running; zero eviction events in `d8-chainsaw-test`; descheduler logs show namespace filtering
- **Fail:** Any pod in `d8-chainsaw-test` is evicted, or descheduler doesn't execute within 5 minutes

## Prerequisites

- Multi-node cluster (minimum 2 nodes)
- Descheduler pre-installed in `d8-descheduler` namespace (with the Deckhouse patch applied)

## How to Run

```bash
# From the e2e directory
task run:exclude-namespaces

# Or directly
chainsaw test --test-dir ./tests/exclude-namespaces-from-processing/
```
