# Descheduler LowNodeUtilization

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates the Kubernetes **Descheduler LowNodeUtilization** plugin correctly rebalances pods across cluster nodes.

**What it does:** Cordons all worker nodes except one, creates a Deployment with 10 replicas (all pods land on the single schedulable node), then uncordons and applies a LowNodeUtilization Descheduler CR. The descheduler evicts pods from the overloaded node, and the Deployment controller recreates them on other nodes.

## Prerequisites

- Multi-node Kubernetes cluster (minimum 3 nodes including master)
- Descheduler pre-installed in the `d8-descheduler` namespace
- Chainsaw CLI installed. See `../../E2E.md` for instructions.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-descheduler-ready` | Asserts descheduler deployment exists and has ready replicas |
| 2 | `check-minimum-nodes` | Verifies cluster has at least 2 worker nodes |
| 3 | `create-imbalanced-workload` | Selects a worker node, cordons others, creates Deployment (cleanup uncordons) |
| 4 | `wait-deployment-ready` | Waits for Deployment Available condition and 10 ready replicas |
| 5 | `assert-pods-on-target-node` | Verifies all pods are concentrated on one node |
| 6 | `uncordon-nodes` | Uncordons all worker nodes so redistribution is possible |
| 7 | `apply-descheduler-cr` | Applies Descheduler CR with LowNodeUtilization strategy (cleanup deletes CR) |
| 8 | `assert-configmap-updated` | Asserts descheduler policy ConfigMap contains the new profile (native assert) |
| 9 | `wait-descheduler-ready` | Waits for descheduler deployment Available condition (native wait) |
| 10 | `wait-for-descheduler-cycle` | Polls descheduler logs for LowNodeUtilization plugin execution |
| 11 | `verify-pod-redistribution` | Verifies pods are redistributed across at least 2 nodes |

**Cleanup:** Step 3 cleanup uncordons all nodes. Step 7 cleanup deletes the Descheduler CR. Test namespace (with the Deployment) is auto-deleted by Chainsaw.

## Files

| File | Purpose |
|------|---------|
| `files/descheduler-cr.yaml` | Descheduler CR with LowNodeUtilization strategy and tuned thresholds |
| `files/pause-deployment.yaml` | Deployment with 10 pause pod replicas (100m CPU, 64Mi memory each) |

## Node Requirements

The test creates a Deployment with 10 replicas, each requesting **100m CPU** and **64Mi memory** (1000m CPU and 640Mi total).

- Nodes must have at least **1000m CPU** and **640Mi memory** allocatable to schedule all 10 pods on the target node
- After uncordoning, at least one other worker node must be **underutilized** (all metrics below thresholds) for the descheduler to trigger evictions
- The test uses a **Deployment** (not bare pods) so that evicted pods are recreated by the controller and rescheduled to other nodes

## Why a Deployment Instead of Bare Pods

Bare pods (without an owner like Deployment/ReplicaSet) are simply deleted when evicted by the descheduler. They are not recreated or rescheduled. A Deployment ensures that evicted pods are automatically recreated by the Deployment controller, allowing the scheduler to place them on less-loaded nodes.

## Cordon Safety

The test temporarily cordons worker nodes (step 3) to force all pods onto one node. This is safe:

- Cordoning only prevents **new** pod scheduling; existing pods continue running undisturbed
- The test explicitly uncordons in step 6 (happy path) and in the step 3 cleanup block (failure path)
- Even if the test process is killed, nodes can be manually uncordoned with `kubectl uncordon <node>`

## Policy Config

- **Thresholds** (underutilized): cpu < 55%, memory < 65%, pods < 50%
- **TargetThresholds** (overutilized): cpu > 70%, memory > 80%, pods > 70%

These thresholds are tuned for typical Deckhouse-managed clusters where worker nodes have 40-50% baseline CPU request utilization from system components.

### How the descheduler decides

The LowNodeUtilization strategy classifies each node:

- **Underutilized**: **ALL** metrics (cpu, memory, pods) must be **below** thresholds
- **Overutilized**: **ANY** metric must be **above** targetThresholds
- **Normal**: everything else (neither underutilized nor overutilized)

Eviction only happens when there is at least one overutilized node AND at least one underutilized node. The descheduler calculates utilization based on resource **requests**, not actual usage.

## Running

```bash
# From the e2e directory
task run:low-node-utilization

# Or directly
chainsaw test --test-dir ./tests/low-node-utilization/
```

## Pass/Fail Criteria

- **Pass:** Descheduler logs show `LowNodeUtilization` execution; pods get evicted and redistributed across at least 2 nodes
- **Fail:** Fewer than 2 worker nodes, descheduler not found, plugin doesn't execute, or pods remain on a single node

## Troubleshooting

### Pods stay on 1 node (0 evictions)

The most common cause is that no node qualifies as underutilized. Check current node utilization:

```bash
kubectl logs -n d8-descheduler -l app=descheduler -c descheduler | grep -i "node has been classified"
```

If all nodes show `usagePercentage` above the thresholds for any metric, raise the thresholds in `files/descheduler-cr.yaml`. Remember: a node is only underutilized when ALL metrics are below thresholds.

### Nodes remain cordoned after test failure

If the test process was killed before cleanup ran:

```bash
kubectl get nodes -o custom-columns='NAME:.metadata.name,SCHEDULABLE:.spec.unschedulable'
kubectl uncordon <node-name>
```

### Descheduler cycle never detected

The test polls logs for up to 7 minutes. If the descheduler pod restarted during the test, logs from the previous instance are lost. Check pod restarts:

```bash
kubectl get pods -n d8-descheduler -l app=descheduler
```
