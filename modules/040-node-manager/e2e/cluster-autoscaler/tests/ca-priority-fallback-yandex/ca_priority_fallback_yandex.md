# Cluster Autoscaler Priority Fallback (Yandex Cloud)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **Priority Expander fallback** on a Yandex Cloud cluster when the highest-priority NodeGroup is broken.

**What it does:** Creates a broken high-priority NodeGroup (`e2e-worker-100` with invalid `imageID`) and a working low-priority group (`e2e-worker-50`). After CA enters backoff on the broken group, verifies fallback to `e2e-worker-50` and pod scheduling on those nodes.

## Prerequisites

- Deckhouse cluster on Yandex Cloud
- Cluster Autoscaler with Priority Expander enabled
- CA container args contain `mcm`
- Existing `YandexInstanceClass` named `worker`
- Chainsaw CLI, `kubectl`, and `jq` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name                                       | Description                                                               |
| ---- | ------------------------------------------ | ------------------------------------------------------------------------- |
| 1    | `assert-cluster-autoscaler-exists`         | Asserts CA deployment is ready (cleanup waits for test nodes removal)     |
| 2    | `assert-ca-uses-mcm-provider`              | Verifies CA args contain `mcm`                                            |
| 3    | `assert-yandexinstanceclass-worker-exists` | Asserts `YandexInstanceClass worker` exists                               |
| 4    | `cleanup-leftover-resources`               | Deletes leftover NodeGroups and instance classes                          |
| 5    | `restart-cluster-autoscaler`               | Rollout restart and wait for readiness                                    |
| 6    | `wait-for-ca-initialization`               | Sleep 15s                                                                 |
| 7    | `create-e2e-worker-small-instanceclass`    | Clones `worker` → `e2e-worker-small` (working IC)                         |
| 8    | `create-broken-yandexinstanceclass`        | Clones `worker` → `e2e-worker-broken` with `imageID: fd8INVALID000000000` |
| 9    | `apply-nodegroup-100-broken`               | Applies `e2e-worker-100` referencing broken IC (priority 100)             |
| 10   | `apply-nodegroup-50`                       | Applies `e2e-worker-50` referencing working IC (priority 50)              |
| 11   | `apply-deployment`                         | Applies shared Deployment (tolerates both taints)                         |
| 12   | `wait-for-deckhouse-processing`            | Sleep 30s                                                                 |
| 13   | `assert-ca-selects-priority-100`           | Polls logs for initial selection of `e2e-worker-100` (up to 5 min)        |
| 14   | `wait-for-ca-backoff-and-fallback`         | Polls logs for backoff + fallback (up to **60 min**, `--since=90m`)       |
| 15   | `assert-pods-running`                      | Asserts 3 ready replicas                                                  |
| 16   | `assert-pods-on-worker-50-nodes`           | Verifies all pods are on `e2e-worker-50` nodes                            |

**Cleanup:** All test NodeGroups, instance classes, and Deployment are deleted via step cleanup blocks.

## Files

| File                                           | Purpose                                                    |
| ---------------------------------------------- | ---------------------------------------------------------- |
| `chainsaw-test.yaml`                           | Chainsaw test definition                                   |
| `manifests/nodegroup-100-broken.yaml`          | High-priority NodeGroup referencing `e2e-worker-broken` IC |
| `../common/manifests/nodegroup-50-yandex.yaml` | Low-priority NodeGroup referencing `e2e-worker-small` IC   |
| `../common/manifests/deployment.yaml`          | 3-replica Deployment tolerating both taints                |

## Differences from DVP Variant

| Aspect              | DVP                               | Yandex             |
| ------------------- | --------------------------------- | ------------------ |
| Broken IC mechanism | Invalid `virtualMachineClassName` | Invalid `imageID`  |
| Fallback timeout    | 30 minutes                        | 60 minutes         |
| Log window          | `--since=30m`                     | `--since=90m`      |
| CA restart timing   | After IC creation                 | Before IC creation |

Yandex provider errors are typically slower to surface, requiring longer polling intervals.

## Running

```bash
# From the test directory
task run

# From cluster-autoscaler root
task ca-priority-fallback-yandex:run
```

## Pass/Fail Criteria

- **Pass:** Logs show initial selection of `e2e-worker-100`, then backoff + fallback to `e2e-worker-50`; 3 pods on `e2e-worker-50` nodes
- **Fail:** Fallback not detected within 60 minutes, pods not ready, or pods on wrong NodeGroup

## Troubleshooting

### Fallback timeout (step 14)

Yandex backoff can take up to 60 minutes. Plan accordingly:

```bash
kubectl logs -n d8-cloud-instance-manager -l app=cluster-autoscaler --all-containers --since=90m | \
  grep -E "e2e-worker-(100|50)"
kubectl get yandexinstanceclass e2e-worker-broken -o yaml
```

### MCM errors not appearing in CA logs

Check MCM pod logs separately — Yandex provisioning errors may appear there before CA marks the group as not ready.
