# Cluster Autoscaler Scale From Zero (DVP)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **Cluster Autoscaler scale-from-zero** with the **Priority Expander** on a DVP / Cluster API cluster.

**What it does:** Creates two NodeGroups (`e2e-worker-100` with priority 100 and `e2e-worker-50` with priority 50), both scaled to zero. Deploys a workload that tolerates only the high-priority group's taint and verifies that CA selects `e2e-worker-100`, scales it up, and schedules all pods there — without creating nodes in the lower-priority group.

## Prerequisites

- Deckhouse cluster with `node-manager` module and CloudEphemeral NodeGroups
- Cluster Autoscaler deployment ready in `d8-cloud-instance-manager`
- CA container args contain `clusterapi` (DVP / CAPI provider)
- Existing `DVPInstanceClass` named `worker` (used as a template for cloning)
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name                                    | Description                                                                     |
| ---- | --------------------------------------- | ------------------------------------------------------------------------------- |
| 1    | `assert-dvpinstanceclass-worker-exists` | Asserts `DVPInstanceClass worker` exists (cleanup waits for test nodes removal) |
| 2    | `cleanup-leftover-resources`            | Deletes leftover NodeGroups and instance class from previous runs               |
| 3    | `create-e2e-worker-small-instanceclass` | Clones `worker` → `e2e-worker-small` with reduced VM specs (cleanup deletes IC) |
| 4    | `apply-nodegroup-100`                   | Applies NodeGroup `e2e-worker-100` (priority 100, minPerZone: 0)                |
| 5    | `apply-nodegroup-50`                    | Applies NodeGroup `e2e-worker-50` (priority 50, minPerZone: 0)                  |
| 6    | `assert-cluster-autoscaler-exists`      | Asserts CA deployment has ready replicas                                        |
| 7    | `assert-ca-uses-clusterapi-provider`    | Verifies CA args contain `clusterapi`                                           |
| 8    | `restart-cluster-autoscaler`            | Rollout restart and wait for CA readiness                                       |
| 9    | `wait-for-ca-initialization`            | Sleep 15s for CA initialization                                                 |
| 10   | `apply-deployment`                      | Applies `e2e-nginx` Deployment (tolerates only `worker-100` taint)              |
| 11   | `wait-for-deckhouse-processing`         | Sleep 30s for Deckhouse reconciliation                                          |
| 12   | `assert-ca-selects-priority-100`        | Polls CA logs for `e2e-worker-100.*chosen as the highest available`             |
| 13   | `assert-pods-running`                   | Asserts Deployment has 3 ready replicas                                         |
| 14   | `assert-pods-on-worker-100-nodes`       | Verifies all pods are on nodes with `node.deckhouse.io/group=e2e-worker-100`    |
| 15   | `assert-worker-50-has-no-nodes`         | Asserts no nodes exist for `e2e-worker-50`                                      |

**Cleanup:** NodeGroups, instance class, and Deployment are deleted via step cleanup blocks. Step 1 cleanup waits up to 15 minutes for nodes with label `app=e2e-autoscaler-test` to disappear.

## Files

| File                                                  | Purpose                                                              |
| ----------------------------------------------------- | -------------------------------------------------------------------- |
| `chainsaw-test.yaml`                                  | Chainsaw test definition                                             |
| `../common/manifests/nodegroup-100-dvp.yaml`          | High-priority NodeGroup (priority 100, taint `dedicated=worker-100`) |
| `../common/manifests/nodegroup-50-dvp.yaml`           | Low-priority NodeGroup (priority 50, taint `dedicated=worker-50`)    |
| `../common/manifests/deployment-scale-from-zero.yaml` | 3-replica Deployment with anti-affinity; tolerates only `worker-100` |
| `../common/asserts/assert-dvp-instanceclass.yaml`     | Asserts `DVPInstanceClass worker` exists                             |
| `../common/asserts/assert-ca-exists.yaml`             | Asserts CA deployment is ready                                       |
| `../common/asserts/assert-deployment-ready.yaml`      | Asserts 3 ready replicas                                             |
| `../common/asserts/assert-no-worker-50-nodes.yaml`    | Error-assert: no nodes for `e2e-worker-50`                           |

## How Priority Expander Works

Deckhouse configures CA with `--expander=priority,least-waste`. NodeGroup priorities from `spec.cloudInstances.priority` are written to ConfigMap `cluster-autoscaler-priority-expander` by the `set_ng_priorities` hook.

The Deployment tolerates only the `e2e-worker-100` taint, so CA must scale that group. The lower-priority group cannot schedule these pods even if scaled up.

## Running

```bash
# From the test directory
task run

# From cluster-autoscaler root
task ca-scale-from-zero-dvp:run

# Or directly
chainsaw test --test-dir . --config ../../chainsaw-config.yaml
```

## Pass/Fail Criteria

- **Pass:** CA logs show `e2e-worker-100` chosen as highest available; 3 pods running on `e2e-worker-100` nodes; no nodes in `e2e-worker-50`
- **Fail:** CA not found, wrong cloud provider (no `clusterapi` in args), scale-up error in logs, pods not ready, or pods scheduled on wrong NodeGroup

## Troubleshooting

### Timeout on priority selection (step 12)

Check CA logs and cloud provider errors:

```bash
kubectl logs -n d8-cloud-instance-manager -l app=cluster-autoscaler -c cluster-autoscaler --tail=200
kubectl get nodegroup e2e-worker-100 e2e-worker-50
kubectl get nodes -l app=e2e-autoscaler-test
```

### `failed to increase node group size` in logs

The test fails fast on this pattern. Check DVP/CAPI MachineDeployment status, quotas, and `DVPInstanceClass e2e-worker-small`.

### Pods Pending

Verify the Deployment tolerates `dedicated=worker-100:NoExecute` and that CA successfully created nodes with taint `dedicated=worker-100`.
