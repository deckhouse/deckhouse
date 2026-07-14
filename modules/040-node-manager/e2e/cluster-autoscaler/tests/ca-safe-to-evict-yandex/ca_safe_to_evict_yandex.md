# Cluster Autoscaler Safe-to-Evict (Yandex Cloud)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates Cluster Autoscaler **scale-down behavior** with the `cluster-autoscaler.kubernetes.io/safe-to-evict: "true"` annotation on a Yandex Cloud (MCM) cluster.

**What it does:** Scales up a node from zero, creates a standalone pod annotated as safe-to-evict, removes the main Deployment, and verifies that CA scales down the node despite the standalone pod still running.

## Prerequisites

- Deckhouse cluster on Yandex Cloud
- Cluster Autoscaler deployment ready in `d8-cloud-instance-manager`
- CA container args contain `mcm`
- Existing `YandexInstanceClass` named `worker`
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name                                       | Description                                                           |
| ---- | ------------------------------------------ | --------------------------------------------------------------------- |
| 1    | `assert-cluster-autoscaler-exists`         | Asserts CA deployment is ready (cleanup waits for test nodes removal) |
| 2    | `assert-ca-uses-mcm-provider`              | Verifies CA args contain `mcm`                                        |
| 3    | `assert-yandexinstanceclass-worker-exists` | Asserts `YandexInstanceClass worker` exists                           |
| 4    | `cleanup-leftover-resources`               | Deletes leftover NodeGroup, IC, and blocking pod                      |
| 5    | `restart-cluster-autoscaler`               | Rollout restart and wait for readiness                                |
| 6    | `wait-for-ca-initialization`               | Sleep 15s                                                             |
| 7    | `create-e2e-worker-small-instanceclass`    | Clones `worker` → `e2e-worker-small`                                  |
| 8    | `apply-nodegroup`                          | Applies NodeGroup `e2e-safe-to-evict` (minPerZone: 0)                 |
| 9    | `apply-deployment`                         | Applies 1-replica Deployment to trigger scale-up                      |
| 10   | `wait-for-deckhouse-processing`            | Sleep 30s                                                             |
| 11   | `assert-pods-running`                      | Asserts Deployment has 1 ready replica                                |
| 12   | `apply-blocking-pod`                       | Creates standalone pod with `safe-to-evict: "true"`                   |
| 13   | `wait-for-blocking-pod-running`            | Waits for blocking pod Running                                        |
| 14   | `delete-deployment-trigger-scale-down`     | Deletes Deployment                                                    |
| 15   | `assert-scale-down-completes`              | Polls until no test nodes (up to 20 min)                              |
| 16   | `assert-no-test-nodes`                     | Error-assert: zero test nodes                                         |

**Note:** Unlike the DVP variant, CA restart happens before instance class creation (Yandex test order).

**Cleanup:** NodeGroup, instance class, Deployment, and blocking pod are deleted via step cleanup blocks.

## Files

| File                                                      | Purpose                                      |
| --------------------------------------------------------- | -------------------------------------------- |
| `chainsaw-test.yaml`                                      | Chainsaw test definition                     |
| `../common/manifests/nodegroup-safe-to-evict-yandex.yaml` | NodeGroup `e2e-safe-to-evict` for Yandex     |
| `../common/manifests/deployment-safe-to-evict.yaml`       | 1-replica Deployment                         |
| `../common/manifests/pod-blocking-safe-to-evict.yaml`     | Standalone pod with safe-to-evict annotation |

## Running

```bash
# From the test directory
task run

# From cluster-autoscaler root
task ca-safe-to-evict-yandex:run
```

## Pass/Fail Criteria

- **Pass:** Node scaled up then removed within 20 minutes after Deployment deletion
- **Fail:** Scale-up fails, blocking pod not Running, or node persists after timeout

## Troubleshooting

### Scale-down timeout

```bash
kubectl logs -n d8-cloud-instance-manager -l app=cluster-autoscaler --all-containers --tail=200
kubectl get nodes -l app=e2e-autoscaler-test
kubectl get pods -n $NAMESPACE -o wide
```

### Wrong cloud provider

If step 2 fails (`grep mcm`), use `ca-safe-to-evict-dvp` for DVP clusters instead.
