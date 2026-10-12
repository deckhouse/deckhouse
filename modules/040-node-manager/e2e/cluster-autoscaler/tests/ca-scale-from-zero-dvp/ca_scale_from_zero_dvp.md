# Cluster Autoscaler Scale From Zero (DVP)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **Cluster Autoscaler scale-from-zero** with the **Priority Expander** on a DVP / Cluster API cluster.

**What it does:** Creates two NodeGroups (`e2e-worker-100` with priority 100 and `e2e-worker-50` with priority 50), both scaled to zero. Deploys a workload that tolerates only the high-priority group's taint and verifies that CA selects `e2e-worker-100`, scales it up, and schedules all pods there — without creating nodes in the lower-priority group.

The test also checks the cloud MachineHealthCheck: before anything is created, that it has the checks which tolerate a bootstrapping node and the remediation limit; after the scale-up, that it replaced no test machine. See [MachineHealthCheck checks](../../README.md#machinehealthcheck-checks-dvp-scenarios).

## Prerequisites

- Deckhouse cluster with `node-manager` module and CloudEphemeral NodeGroups
- Cluster Autoscaler deployment ready in `d8-cloud-instance-manager`
- CA container args contain `clusterapi` (DVP / CAPI provider)
- Existing `DVPInstanceClass` named `worker` (used as a template for cloning)
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name                                         | Description                                                                             |
| ---- | -------------------------------------------- | --------------------------------------------------------------------------------------- |
| 1    | `assert-dvpinstanceclass-worker-exists`      | Asserts `DVPInstanceClass worker` exists (cleanup waits for test nodes removal)         |
| 2    | `assert-cloud-machine-health-check`          | Polls up to 6 min for the fixed cloud MachineHealthCheck (Ready=False 600s, 50% limit) |
| 3    | `cleanup-leftover-resources`                 | Deletes leftover NodeGroups and instance class from previous runs                       |
| 4    | `create-e2e-worker-small-instanceclass`      | Clones `worker` → `e2e-worker-small` with reduced VM specs (cleanup deletes IC)         |
| 5    | `apply-nodegroup-100`                        | Applies NodeGroup `e2e-worker-100` (priority 100, minPerZone: 0)                        |
| 6    | `apply-nodegroup-50`                         | Applies NodeGroup `e2e-worker-50` (priority 50, minPerZone: 0)                          |
| 7    | `assert-cluster-autoscaler-exists`           | Asserts CA deployment has ready replicas                                                |
| 8    | `assert-ca-uses-clusterapi-provider`         | Verifies CA args contain `clusterapi`                                                   |
| 9    | `restart-cluster-autoscaler`                 | Rollout restart and wait for CA readiness                                               |
| 10   | `wait-for-ca-initialization`                 | Sleep 15s for CA initialization                                                         |
| 11   | `apply-deployment`                           | Applies `e2e-nginx` Deployment (tolerates only `worker-100` taint)                      |
| 12   | `wait-for-deckhouse-processing`              | Sleep 30s for Deckhouse reconciliation                                                  |
| 13   | `assert-ca-selects-priority-100`             | Polls CA logs for `e2e-worker-100.*chosen as the highest available`                     |
| 14   | `assert-pods-running`                        | Asserts Deployment has 3 ready replicas                                                 |
| 15   | `assert-no-machine-health-check-remediation` | Asserts MachineHealthCheck marked no test machine unhealthy during the test             |
| 16   | `assert-pods-on-worker-100-nodes`            | Verifies all pods are on nodes with `node.deckhouse.io/group=e2e-worker-100`            |
| 17   | `assert-worker-50-has-no-nodes`              | Asserts no nodes exist for `e2e-worker-50`                                              |

**Cleanup:** NodeGroups, instance class, and Deployment are deleted via step cleanup blocks. Step 1 cleanup waits up to 15 minutes for nodes with label `app=e2e-autoscaler-test` to disappear.

## Files

| File                                                                       | Purpose                                                               |
| -------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| `chainsaw-test.yaml`                                                       | Chainsaw test definition                                              |
| `../common/manifests/nodegroup-100-dvp.yaml`                               | High-priority NodeGroup (priority 100, taint `dedicated=worker-100`)  |
| `../common/manifests/nodegroup-50-dvp.yaml`                                | Low-priority NodeGroup (priority 50, taint `dedicated=worker-50`)     |
| `../common/manifests/deployment-scale-from-zero.yaml`                      | 3-replica Deployment with anti-affinity; tolerates only `worker-100`  |
| `../common/asserts/assert-dvp-instanceclass.yaml`                          | Asserts `DVPInstanceClass worker` exists                              |
| `../common/asserts/assert-ca-exists.yaml`                                  | Asserts CA deployment is ready                                        |
| `../common/asserts/assert-no-worker-50-nodes.yaml`                         | Error-assert: no nodes for `e2e-worker-50`                            |
| `../common/step-templates/assert-cloud-machine-health-check.yaml`          | Step template: cloud MachineHealthCheck checks and remediation limit  |
| `../common/step-templates/assert-no-machine-health-check-remediation.yaml` | Step template: no test machine marked unhealthy by MachineHealthCheck |

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

- **Pass:** CA logs show `e2e-worker-100` chosen as highest available; 3 pods running on `e2e-worker-100` nodes; no nodes in `e2e-worker-50`; the cloud MachineHealthCheck has the fixed checks, and MachineHealthCheck replaced no test machine
- **Fail:** CA not found, wrong cloud provider (no `clusterapi` in args), scale-up error in logs, pods not ready, pods scheduled on wrong NodeGroup, the cloud MachineHealthCheck has the old checks, or MachineHealthCheck marked a test machine unhealthy

## Troubleshooting

### Cloud MachineHealthCheck has the old checks (step 2)

node-controller did not apply the fixed cloud MachineHealthCheck; the step prints its spec and field managers. See [MachineHealthCheck checks](../../README.md#machinehealthcheck-checks-dvp-scenarios).

### MachineHealthCheck replaced a test machine (step 15)

MachineHealthCheck marked a machine of the test NodeGroups unhealthy while it was bootstrapping; the step prints the events and the MachineHealthCheck status. See [MachineHealthCheck checks](../../README.md#machinehealthcheck-checks-dvp-scenarios).

### Timeout on priority selection (step 13)

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
