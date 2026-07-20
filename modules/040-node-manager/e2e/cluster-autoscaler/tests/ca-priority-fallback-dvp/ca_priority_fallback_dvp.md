# Cluster Autoscaler Priority Fallback (DVP)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **Priority Expander fallback** on a DVP / Cluster API cluster when the highest-priority NodeGroup is broken.

**What it does:** Creates a broken high-priority NodeGroup (`e2e-worker-100` with invalid `DVPInstanceClass`) and a working low-priority group (`e2e-worker-50`). Deploys a workload tolerating both taints. CA first attempts the high-priority group, enters backoff after failures, then falls back to `e2e-worker-50`.

## Prerequisites

- Deckhouse cluster with DVP / CAPI cloud provider
- Cluster Autoscaler with Priority Expander enabled
- CA container args contain `clusterapi`
- Existing `DVPInstanceClass` named `worker`
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name                                    | Description                                                                                       |
| ---- | --------------------------------------- | ------------------------------------------------------------------------------------------------- |
| 1    | `assert-dvpinstanceclass-worker-exists` | Asserts `DVPInstanceClass worker` exists (cleanup waits for test nodes removal)                   |
| 2    | `cleanup-leftover-resources`            | Deletes leftover NodeGroups and instance classes                                                  |
| 3    | `create-e2e-worker-small-instanceclass` | Clones `worker` → `e2e-worker-small` (working IC)                                                 |
| 4    | `create-broken-dvpinstanceclass`        | Clones `worker` → `e2e-worker-broken` with `virtualMachineClassName: DOES-NOT-EXIST`              |
| 5    | `apply-nodegroup-100-broken`            | Applies `e2e-worker-100` referencing broken IC (priority 100)                                     |
| 6    | `apply-nodegroup-50`                    | Applies `e2e-worker-50` referencing working IC (priority 50)                                      |
| 7    | `assert-cluster-autoscaler-exists`      | Asserts CA deployment is ready                                                                    |
| 8    | `assert-ca-uses-clusterapi-provider`    | Verifies CA args contain `clusterapi`                                                             |
| 9    | `restart-cluster-autoscaler`            | Rollout restart and wait for readiness                                                            |
| 10   | `wait-for-ca-initialization`            | Sleep 15s                                                                                         |
| 11   | `apply-deployment`                      | Applies shared Deployment (tolerates both `worker-100` and `worker-50` taints)                    |
| 12   | `wait-for-deckhouse-processing`         | Sleep 30s                                                                                         |
| 13   | `assert-ca-selects-priority-100`        | Polls logs for initial selection of `e2e-worker-100` (up to 5 min)                                |
| 14   | `wait-for-ca-backoff-and-fallback`      | Polls logs for `e2e-worker-100.*not ready for scaleup` AND `e2e-worker-50.*chosen` (up to 30 min) |
| 15   | `assert-pods-running`                   | Asserts 3 ready replicas                                                                          |
| 16   | `assert-pods-on-worker-50-nodes`        | Verifies all pods are on `e2e-worker-50` nodes                                                    |

**Cleanup:** All test NodeGroups, instance classes, and Deployment are deleted via step cleanup blocks.

## Files

| File                                        | Purpose                                                            |
| ------------------------------------------- | ------------------------------------------------------------------ |
| `chainsaw-test.yaml`                        | Chainsaw test definition                                           |
| `manifests/nodegroup-100-broken.yaml`       | High-priority NodeGroup referencing `e2e-worker-broken` IC         |
| `../common/manifests/nodegroup-50-dvp.yaml` | Low-priority NodeGroup referencing `e2e-worker-small` IC           |
| `../common/manifests/deployment.yaml`       | 3-replica Deployment tolerating both taints with pod anti-affinity |
| `../common/asserts/*`                       | Shared CA and deployment assertions                                |

## How Fallback Works

1. CA evaluates NodeGroups by priority — `e2e-worker-100` is selected first
2. Scale-up fails because `e2e-worker-broken` references a non-existent VM class
3. After backoff period, CA marks `e2e-worker-100` as "not ready for scaleup"
4. CA falls back to `e2e-worker-50` (next highest available priority)
5. Working nodes are created and pods are scheduled

## Running

```bash
# From the test directory
task run

# From cluster-autoscaler root
task ca-priority-fallback-dvp:run
```

## Pass/Fail Criteria

- **Pass:** Logs show initial selection of `e2e-worker-100`, then backoff + fallback to `e2e-worker-50`; 3 pods running on `e2e-worker-50` nodes
- **Fail:** Fallback not detected within 30 minutes, pods not ready, or pods on `e2e-worker-100` nodes

## Troubleshooting

### Fallback timeout (step 14)

Backoff can take up to 30 minutes on DVP. Check that the broken IC actually prevents VM creation:

```bash
kubectl get dvpinstanceclass e2e-worker-broken -o yaml
kubectl logs -n d8-cloud-instance-manager -l app=cluster-autoscaler --all-containers --since=30m | \
  grep -E "e2e-worker-(100|50)"
```

### Pods on wrong NodeGroup

If `e2e-worker-100` somehow creates nodes (broken IC not broken enough), the test may pass step 13 but fail step 16. Verify `virtualMachineClassName: DOES-NOT-EXIST` in the broken IC.
