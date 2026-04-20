# Descheduler HighNodeUtilization

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates the Kubernetes **Descheduler HighNodeUtilization** plugin executes correctly. The plugin evicts pods from **underutilized** nodes so the scheduler can pack them onto busier nodes — freeing up entire nodes for cost savings or cluster autoscaler downscaling.

**What it does:** Creates a Deployment with topology spread constraints to distribute pods across worker nodes, applies a HighNodeUtilization Descheduler CR, and verifies the plugin executes.

## Prerequisites

- Multi-node Kubernetes cluster (minimum 3 nodes including master)
- Descheduler pre-installed in the `d8-descheduler` namespace
- Chainsaw CLI installed. See `../../E2E.md` for instructions.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-descheduler-ready` | Asserts descheduler deployment exists and has ready replicas |
| 2 | `check-minimum-nodes` | Verifies cluster has at least 2 worker nodes |
| 3 | `create-spread-workload` | Creates Deployment with topology spread across worker nodes |
| 4 | `wait-deployment-ready` | Waits for Deployment Available condition and 3 ready replicas |
| 5 | `assert-pods-spread` | Verifies pods are distributed across at least 2 nodes |
| 6 | `apply-descheduler-cr` | Applies Descheduler CR with HighNodeUtilization strategy (cleanup deletes CR) |
| 7 | `assert-configmap-updated` | Asserts descheduler policy ConfigMap contains the new profile (native assert) |
| 8 | `wait-descheduler-ready` | Waits for descheduler deployment Available condition (native wait) |
| 9 | `wait-for-descheduler-cycle` | Polls descheduler logs for HighNodeUtilization plugin execution |

**Note:** The workload is created BEFORE the descheduler CR to ensure pods are stable before eviction starts.

**Cleanup:** Step 6 cleanup deletes the Descheduler CR. Test namespace (with the Deployment) is auto-deleted by Chainsaw.

## Files

| File | Purpose |
|------|---------|
| `files/descheduler-cr.yaml` | Descheduler CR with HighNodeUtilization strategy and tuned thresholds |
| `files/spread-deployment.yaml` | Deployment with 3 pause pod replicas and topology spread constraints |

## Policy Config

- **Thresholds** (underutilized): cpu < 55%, memory < 65%, pods < 50%

Nodes with ALL metrics below these thresholds are classified as underutilized and become candidates for pod eviction.

### How the descheduler decides

The HighNodeUtilization strategy is the **opposite** of LowNodeUtilization:

- **Underutilized**: **ALL** metrics (cpu, memory, pods) must be **below** thresholds — pods are evicted FROM these nodes
- The goal is to consolidate workloads onto fewer, busier nodes
- Evicted pods are rescheduled by the scheduler (ideally to already-busy nodes)

The descheduler calculates utilization based on resource **requests**, not actual usage.

## Limitations

- The test verifies that the HighNodeUtilization plugin **executes**, but does not verify actual pod consolidation. Consolidation depends on the scheduler's scoring strategy (`MostAllocated` is recommended but not guaranteed).
- With the default scheduler (`LeastAllocated` scoring), evicted pods may be rescheduled back to the same underutilized nodes, creating a cycle.

## Running

```bash
# From the e2e directory
task run:high-node-utilization

# Or directly
chainsaw test --test-dir ./tests/high-node-utilization/
```

## Pass/Fail Criteria

- **Pass:** HighNodeUtilization plugin executes (found in descheduler logs); pods are initially spread across multiple nodes
- **Fail:** Fewer than 2 worker nodes, descheduler not found, or plugin doesn't execute within 7 minutes

## Troubleshooting

**Plugin executes but 0 evictions**

No node is classified as underutilized. Check node classification:

```bash
kubectl logs -n d8-descheduler -l app=descheduler -c descheduler | grep -i "node has been classified"
```

If all nodes show `usagePercentage` above thresholds for any metric, raise the thresholds in `files/descheduler-cr.yaml`. A node is only underutilized when ALL metrics are below thresholds.

**Pods stuck in Pending**

The topology spread constraint `whenUnsatisfiable: DoNotSchedule` prevents scheduling if it would violate `maxSkew: 1`. This happens when there are fewer schedulable nodes than replicas. Ensure at least 2 worker nodes are available.
