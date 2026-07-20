# Cluster Autoscaler Safe-to-Evict (DVP)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates Cluster Autoscaler **scale-down behavior** with the `cluster-autoscaler.kubernetes.io/safe-to-evict: "true"` annotation on a DVP / Cluster API cluster.

**What it does:** Scales up a node from zero via a Deployment, then creates a standalone pod annotated as safe-to-evict. After deleting the Deployment (making the node underutilized), verifies that CA scales down and removes the node despite the standalone pod still running on it.

## Prerequisites

- Deckhouse cluster with DVP / CAPI cloud provider
- Cluster Autoscaler deployment ready in `d8-cloud-instance-manager`
- CA container args contain `clusterapi`
- Existing `DVPInstanceClass` named `worker`
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name                                    | Description                                                                     |
| ---- | --------------------------------------- | ------------------------------------------------------------------------------- |
| 1    | `assert-dvpinstanceclass-worker-exists` | Asserts `DVPInstanceClass worker` exists (cleanup waits for test nodes removal) |
| 2    | `cleanup-leftover-resources`            | Deletes leftover NodeGroup, IC, and blocking pod                                |
| 3    | `create-e2e-worker-small-instanceclass` | Clones `worker` → `e2e-worker-small`                                            |
| 4    | `apply-nodegroup`                       | Applies NodeGroup `e2e-safe-to-evict` (minPerZone: 0)                           |
| 5    | `assert-cluster-autoscaler-exists`      | Asserts CA deployment is ready                                                  |
| 6    | `assert-ca-uses-clusterapi-provider`    | Verifies CA args contain `clusterapi`                                           |
| 7    | `restart-cluster-autoscaler`            | Rollout restart and wait for readiness                                          |
| 8    | `wait-for-ca-initialization`            | Sleep 15s                                                                       |
| 9    | `apply-deployment`                      | Applies 1-replica Deployment to trigger scale-up                                |
| 10   | `wait-for-deckhouse-processing`         | Sleep 30s                                                                       |
| 11   | `assert-pods-running`                   | Asserts Deployment has 1 ready replica (node scaled up)                         |
| 12   | `apply-blocking-pod`                    | Creates standalone pod with `safe-to-evict: "true"` annotation                  |
| 13   | `wait-for-blocking-pod-running`         | Waits for blocking pod to reach Running phase                                   |
| 14   | `delete-deployment-trigger-scale-down`  | Deletes Deployment to make node underutilized                                   |
| 15   | `assert-scale-down-completes`           | Polls until no nodes with `app=e2e-autoscaler-test` (up to 20 min)              |
| 16   | `assert-no-test-nodes`                  | Error-assert: confirms zero test nodes remain                                   |

**Note:** Normally, standalone pods (without a controller) block CA scale-down. The `safe-to-evict: "true"` annotation tells CA the pod can be evicted during scale-down.

**Cleanup:** NodeGroup, instance class, Deployment, and blocking pod are deleted via step cleanup blocks.

## Files

| File                                                   | Purpose                                                                   |
| ------------------------------------------------------ | ------------------------------------------------------------------------- |
| `chainsaw-test.yaml`                                   | Chainsaw test definition                                                  |
| `../common/manifests/nodegroup-safe-to-evict-dvp.yaml` | Single NodeGroup `e2e-safe-to-evict` with taint `dedicated=safe-to-evict` |
| `../common/manifests/deployment-safe-to-evict.yaml`    | 1-replica Deployment to trigger initial scale-up                          |
| `../common/manifests/pod-blocking-safe-to-evict.yaml`  | Standalone pod with `safe-to-evict: "true"` annotation                    |
| `../common/asserts/assert-deployment-ready-1.yaml`     | Asserts 1 ready replica                                                   |
| `../common/asserts/assert-no-test-nodes.yaml`          | Error-assert: no test nodes                                               |

## How Safe-to-Evict Works

By default, CA treats standalone pods as blocking scale-down because evicting them would permanently remove the workload (no controller to recreate them). The annotation `cluster-autoscaler.kubernetes.io/safe-to-evict: "true"` overrides this — CA may evict the pod during scale-down.

This test verifies the full cycle: scale-up → add annotated pod → remove main workload → scale-down proceeds.

## Running

```bash
# From the test directory
task run

# From cluster-autoscaler root
task ca-safe-to-evict-dvp:run
```

## Pass/Fail Criteria

- **Pass:** Node is scaled up (1 pod running), then removed within 20 minutes after Deployment deletion, despite standalone pod presence
- **Fail:** Scale-up fails, blocking pod not Running, or node still present after 20 minutes

## Troubleshooting

### Scale-down timeout (step 15)

Check if CA recognizes the safe-to-evict annotation:

```bash
kubectl get pod -n $NAMESPACE e2e-blocking-pod -o yaml | grep safe-to-evict
kubectl logs -n d8-cloud-instance-manager -l app=cluster-autoscaler -c cluster-autoscaler --tail=200 | \
  grep -i "safe-to-evict\|unneeded\|scale-down"
kubectl get nodes -l app=e2e-autoscaler-test -o wide
```

### Node not scaling up (step 11)

Verify NodeGroup `e2e-safe-to-evict` was created and CA detected pending pods:

```bash
kubectl get nodegroup e2e-safe-to-evict
kubectl get pods -n $NAMESPACE -o wide
```

### Blocking pod stuck Pending

Check taint toleration — the pod must tolerate `dedicated=safe-to-evict:NoExecute`.
