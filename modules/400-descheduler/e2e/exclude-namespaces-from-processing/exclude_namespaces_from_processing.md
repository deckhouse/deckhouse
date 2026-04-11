# Descheduler Exclude Namespaces from Processing — Chainsaw Test Summary

## What it does

This [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test validates the Deckhouse descheduler patch (`001-filter-pods-in-deckhouse-namespaces.patch`) that prevents eviction of pods in `d8-*` and `kube-system` namespaces. The patch adds a constraint to `DefaultEvictor` that rejects pods from protected namespaces.

## Test steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `check-prerequisites` | Verifies cluster has 2+ nodes and descheduler deployment exists in `d8-descheduler` namespace |
| 2 | `configure-descheduler` | Backs up existing policy, applies LowNodeUtilization config to trigger evictions, restarts descheduler |
| 3 | `create-workloads` | Creates 5 pods in `d8-chainsaw-test` (protected) and 5 pods in regular namespace, all pinned to one node |
| 4 | `verify-namespace-exclusion` | Waits for descheduler cycle, checks logs for filtering messages, verifies protected pods are NOT evicted |

**Cleanup:** `finally` block deletes `d8-chainsaw-test` namespace and restores the original descheduler policy.

## Pass/Fail criteria

- **Pass:** Descheduler logs contain `"pod in the deckhouse namespace"` filtering messages; all 5 protected pods remain running; zero eviction events in `d8-chainsaw-test`
- **Fail:** Any pod in `d8-chainsaw-test` is evicted, or descheduler doesn't execute within 5 minutes

## Prerequisites

- Multi-node cluster (minimum 2 nodes)
- Descheduler pre-installed in `d8-descheduler` namespace (with the Deckhouse patch applied)

## How to run

```bash
chainsaw test --test-dir ./exclude-namespaces-from-processing/
```
