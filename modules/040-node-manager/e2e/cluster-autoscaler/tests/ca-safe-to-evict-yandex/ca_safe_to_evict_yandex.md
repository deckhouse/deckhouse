# Cluster Autoscaler Safe-to-Evict (Yandex Cloud)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates Cluster Autoscaler **scale-down behavior** with the `cluster-autoscaler.kubernetes.io/safe-to-evict: "true"` annotation on a Yandex Cloud (MCM) cluster.

**What it does:** Scales up a node from zero, creates a standalone pod annotated as safe-to-evict that prefers the test node but can also run on another node, removes the main Deployment, and verifies that CA scales down the node despite the standalone pod still running.

## Prerequisites

- Deckhouse cluster on Yandex Cloud
- Cluster Autoscaler deployment ready in `d8-cloud-instance-manager`
- Test NodeGroups are created with `node.deckhouse.io/use-mcm: "true"`, so they run on MCM even when Yandex NodeGroups default to CAPI. Their CA is the Deployment with `--cloud-provider=mcm` (`cluster-autoscaler`, or `cluster-autoscaler-mcm` next to a CAPI one); the test finds it after creating the NodeGroups
- Existing `YandexInstanceClass` named `worker`
- A Ready, schedulable node outside the test NodeGroup (for example, from the base `worker` NodeGroup) without `NoSchedule`/`NoExecute` taints and with room for a 10m CPU / 16Mi pod
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name                                       | Description                                                                                      |
| ---- | ------------------------------------------ | ------------------------------------------------------------------------------------------------ |
| 1    | `assert-yandexinstanceclass-worker-exists` | Asserts `YandexInstanceClass worker` exists (cleanup waits for test nodes removal)               |
| 2    | `cleanup-leftover-resources`               | Deletes leftover NodeGroup, IC, and blocking pod                                                 |
| 3    | `create-e2e-worker-small-instanceclass`    | Clones `worker` → `e2e-worker-small`                                                             |
| 4    | `apply-nodegroup`                          | Applies NodeGroup `e2e-safe-to-evict` (minPerZone: 0, `use-mcm`)                                 |
| 5    | `assert-mcm-cluster-autoscaler-ready`      | Waits for the CA Deployment with `--cloud-provider=mcm` to list the test NodeGroups and roll out |
| 6    | `restart-cluster-autoscaler`               | Rollout restart of that Deployment and wait for readiness                                        |
| 7    | `wait-for-ca-initialization`               | Sleep 15s for CA initialization                                                                  |
| 8    | `apply-deployment`                         | Applies 1-replica Deployment to trigger scale-up                                                 |
| 9    | `wait-for-deckhouse-processing`            | Sleep 30s                                                                                        |
| 10   | `assert-pods-running`                      | Asserts Deployment has 1 ready replica                                                           |
| 11   | `apply-blocking-pod`                       | Creates standalone pod with `safe-to-evict: "true"`                                              |
| 12   | `wait-for-blocking-pod-running`            | Waits for blocking pod Running                                                                   |
| 13   | `delete-deployment-trigger-scale-down`     | Deletes Deployment                                                                               |
| 14   | `assert-scale-down-completes`              | Polls until no test nodes (up to 20 min)                                                         |
| 15   | `assert-no-test-nodes`                     | Error-assert: zero test nodes                                                                    |

**Note:** The CA Deployment is looked up by `--cloud-provider=mcm`: `cluster-autoscaler` on a cluster with only MCM NodeGroups, `cluster-autoscaler-mcm` when the provider also runs CAPI NodeGroups.

**Cleanup:** NodeGroup, instance class, Deployment, and blocking pod are deleted via step cleanup blocks.

## Files

| File                                                      | Purpose                                                                              |
| --------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| `chainsaw-test.yaml`                                      | Chainsaw test definition                                                             |
| `../common/manifests/nodegroup-safe-to-evict-yandex.yaml` | NodeGroup `e2e-safe-to-evict` for Yandex                                             |
| `../common/manifests/deployment-safe-to-evict.yaml`       | 1-replica Deployment                                                                 |
| `../common/manifests/pod-blocking-safe-to-evict.yaml`     | Standalone pod with safe-to-evict annotation and preferred affinity to the test node |

## How Safe-to-Evict Works

The `safe-to-evict: "true"` annotation lets CA evict a standalone pod, but it does not skip the scheduling check. Before removing a node, CA simulates placing each of its pods on the remaining nodes. A pod that fits only on the node being removed keeps that node (`can reschedule only 0 out of 1 pods` in the CA log). Therefore the blocking pod uses preferred node affinity to `app=e2e-autoscaler-test` instead of a `nodeSelector`. Step `wait-for-blocking-pod-running` also asserts that the pod runs on the test node and that a node outside the test NodeGroup can host it.

## Running

```bash
# From the test directory
task run

# From cluster-autoscaler root
task ca-safe-to-evict-yandex:run
```

## Pass/Fail Criteria

- **Pass:** Node scaled up then removed within 20 minutes after Deployment deletion
- **Fail:** Scale-up fails, blocking pod not Running on the test node, no node outside the test NodeGroup can host it, or node persists after timeout

## Troubleshooting

### Scale-down timeout

```bash
kubectl logs -n d8-cloud-instance-manager -l app=cluster-autoscaler --all-containers --tail=200
kubectl get nodes -l app=e2e-autoscaler-test
kubectl get pods -n $NAMESPACE -o wide
```

`can reschedule only 0 out of 1 pods` means that no other node can take the blocking pod. Check the taints, cordons, and free capacity of nodes outside the test NodeGroup.

### Wrong cloud provider

If step 2 fails (`grep mcm`), use `ca-safe-to-evict-dvp` for DVP clusters instead.
