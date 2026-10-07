# Cluster Autoscaler Scale From Zero (Yandex Cloud)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **Cluster Autoscaler scale-from-zero** with the **Priority Expander** on a Yandex Cloud cluster (MCM provider).

**What it does:** Creates two NodeGroups with different priorities, deploys a workload tolerating only the high-priority taint, and verifies that CA selects `e2e-worker-100`, scales it up, and schedules all pods there — without creating nodes in `e2e-worker-50`.

## Prerequisites

- Deckhouse cluster on Yandex Cloud with CloudEphemeral NodeGroups
- Cluster Autoscaler deployment ready in `d8-cloud-instance-manager`
- Test NodeGroups are created with `node.deckhouse.io/use-mcm: "true"`, so they run on MCM even when Yandex NodeGroups default to CAPI. Their CA is the Deployment with `--cloud-provider=mcm` (`cluster-autoscaler`, or `cluster-autoscaler-mcm` next to a CAPI one); the test finds it after creating the NodeGroups
- Existing `YandexInstanceClass` named `worker`
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name                                       | Description                                                                                      |
| ---- | ------------------------------------------ | ------------------------------------------------------------------------------------------------ |
| 1    | `assert-yandexinstanceclass-worker-exists` | Asserts `YandexInstanceClass worker` exists (cleanup waits for test nodes removal)               |
| 2    | `cleanup-leftover-resources`               | Deletes leftover NodeGroups and instance class from previous runs                                |
| 3    | `create-e2e-worker-small-instanceclass`    | Clones `worker` → `e2e-worker-small` (2 cores, 4Gi RAM, 30GB disk)                               |
| 4    | `apply-nodegroup-100`                      | Applies NodeGroup `e2e-worker-100` (priority 100, minPerZone: 0, `use-mcm`)                      |
| 5    | `apply-nodegroup-50`                       | Applies NodeGroup `e2e-worker-50` (priority 50, minPerZone: 0, `use-mcm`)                        |
| 6    | `assert-mcm-cluster-autoscaler-ready`      | Waits for the CA Deployment with `--cloud-provider=mcm` to list the test NodeGroups and roll out |
| 7    | `restart-cluster-autoscaler`               | Rollout restart of that Deployment and wait for readiness                                        |
| 8    | `wait-for-ca-initialization`               | Sleep 15s for CA initialization                                                                  |
| 9    | `apply-deployment`                         | Applies `e2e-nginx` Deployment (tolerates only `worker-100` taint)                               |
| 10   | `wait-for-deckhouse-processing`            | Sleep 30s for Deckhouse reconciliation                                                           |
| 11   | `assert-ca-selects-priority-100`           | Polls CA logs (`--all-containers --since=10m`) for priority selection                            |
| 12   | `assert-pods-running`                      | Asserts Deployment has 3 ready replicas                                                          |
| 13   | `assert-pods-on-worker-100-nodes`          | Verifies all pods are on `e2e-worker-100` nodes                                                  |
| 14   | `assert-worker-50-has-no-nodes`            | Asserts no nodes exist for `e2e-worker-50`                                                       |

**Note:** The CA Deployment is looked up by `--cloud-provider=mcm`: `cluster-autoscaler` on a cluster with only MCM NodeGroups, `cluster-autoscaler-mcm` when the provider also runs CAPI NodeGroups.

**Cleanup:** NodeGroups, instance class, and Deployment are deleted via step cleanup blocks.

## Files

| File                                                  | Purpose                                                              |
| ----------------------------------------------------- | -------------------------------------------------------------------- |
| `chainsaw-test.yaml`                                  | Chainsaw test definition                                             |
| `../common/manifests/nodegroup-100-yandex.yaml`       | High-priority NodeGroup                                              |
| `../common/manifests/nodegroup-50-yandex.yaml`        | Low-priority NodeGroup                                               |
| `../common/manifests/deployment-scale-from-zero.yaml` | 3-replica Deployment with anti-affinity; tolerates only `worker-100` |
| `../common/asserts/assert-yandex-instanceclass.yaml`  | Asserts `YandexInstanceClass worker` exists                          |
| `../common/asserts/assert-ca-exists.yaml`             | Asserts CA deployment is ready                                       |
| `../common/asserts/assert-no-worker-50-nodes.yaml`    | Error-assert: no nodes for `e2e-worker-50`                           |

## Differences from DVP Variant

| Aspect            | DVP                           | Yandex                         |
| ----------------- | ----------------------------- | ------------------------------ |
| CA provider       | `clusterapi`                  | `mcm`                          |
| Instance class    | `DVPInstanceClass`            | `YandexInstanceClass`          |
| CA restart timing | After instance class creation | Before instance class creation |
| Log reading       | `-c cluster-autoscaler` only  | `--all-containers --since=10m` |

## Running

```bash
# From the test directory
task run

# From cluster-autoscaler root
task ca-scale-from-zero-yandex:run

# Or directly
chainsaw test --test-dir . --config ../../chainsaw-config.yaml
```

## Pass/Fail Criteria

- **Pass:** CA logs show `e2e-worker-100` chosen as highest available; 3 pods running on `e2e-worker-100` nodes; no nodes in `e2e-worker-50`
- **Fail:** CA not found, wrong provider (no `mcm` in args), scale-up error, pods not ready, or pods on wrong NodeGroup

## Troubleshooting

### Wrong scenario for your cloud

If `grep mcm` fails in step 2, you are running the Yandex test on a non-Yandex cluster. Use `ca-scale-from-zero-dvp` instead.

### Timeout on priority selection

Yandex VM provisioning can be slow. Check MCM logs and cloud quotas:

```bash
kubectl logs -n d8-cloud-instance-manager -l app=cluster-autoscaler --all-containers --tail=200
kubectl get nodegroup e2e-worker-100 -o yaml
```
