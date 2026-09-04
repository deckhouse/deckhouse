# Descheduler — EvictionsInBackground feature gate

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates the module always runs the descheduler with the upstream **`EvictionsInBackground`** feature gate, that the descheduler stays healthy with it, and that the gate does not change eviction behavior when no controller implements an eviction-in-background policy.

**What it does:** First asserts the gate is present both in the `descheduler` Deployment spec and in the args of the running pod, while the Deployment reports a ready replica. Then creates a 3-replica StatefulSet whose pods carry the `descheduler.alpha.kubernetes.io/request-evict-only` annotation, concentrated on one worker node (via `spec.nodeName`, no cordoning), applies a `removeDuplicates` Descheduler CR scoped to those pods, and asserts they are evicted and spread across the cluster exactly as unannotated pods would be.

## Why the health check proves something

The module renders the `descheduler` Deployment only when at least one `Descheduler` CR exists, so the gate is checked after this test creates its own CR — the scenario needs no pre-existing descheduler in the cluster.

The descheduler parses `--feature-gates` at startup and **exits with an error on an unknown or malformed gate**. A ready pod whose args contain `EvictionsInBackground=true` is therefore proof that the gate name is valid for the shipped descheduler version — this test fails loudly if an upstream bump ever renames or removes the gate.

## Why the annotated pods must still be evicted

The gate only takes effect when the controller owning the pod answers the eviction API call with `429 Too Many Requests` **and** an error message containing `Eviction triggered evacuation` (this is what KubeVirt does for pods backing virtual machines). Only then does the descheduler record the pod as an eviction in progress instead of an immediate eviction.

No such controller exists in a plain Deckhouse cluster, so the eviction API call succeeds and the pod is evicted normally. This scenario is the regression guard for the "no-op for clusters that have no such workloads" contract stated in the module documentation.

## Prerequisites

- Multi-node Kubernetes cluster (at least **2** schedulable worker nodes)
- Descheduler pre-installed in the `d8-descheduler` namespace
- Deckhouse **ClusterAdmin**-level rights to create `Descheduler` CRs (a plain `kubernetes-admin` identity is denied)
- Chainsaw CLI installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-module-installed` | Asserts the `deschedulers.deckhouse.io` CRD exists |
| 2 | `check-minimum-nodes` | Verifies ≥2 Ready, schedulable, non-control-plane worker nodes (`x_k8s_list` + JMESPath) |
| 3 | `create-statefulset` | Creates a 3-replica annotated StatefulSet pinned to one node via `spec.nodeName` |
| 4 | `wait-statefulset-pinned` | Waits until all 3 pods are Running on the target node **and carry the annotation** |
| 5 | `release-node-pinning` | Patches the template to clear `nodeName` (OnDelete keeps running pods); asserts all 3 pods stay put |
| 6 | `apply-descheduler-cr` | Applies the `removeDuplicates` Descheduler CR scoped by `podLabelSelector` (cleanup deletes it) |
| 7 | `assert-policy-rendered` | Asserts the `descheduler-policy` ConfigMap contains the `e2e-evictions-in-background` profile |
| 8 | `wait-descheduler-rollout` | Waits for the descheduler rollout to finish and the running pod to carry the new policy checksum |
| 9 | `assert-feature-gate-in-deployment` | Asserts the `descheduler` container args contain `--feature-gates` and `EvictionsInBackground=true` |
| 10 | `assert-descheduler-healthy-with-feature-gate` | Asserts the Deployment has a ready replica and the **running pod's** args carry the gate |
| 11 | `verify-annotated-pods-evicted` | Asserts the 3 annotated pods spread across ≥2 nodes and the STS returns to 3/3 ready |
| 12 | `verify-eviction-events` | Asserts a `RemoveDuplicates` eviction event exists for a `test-sts-*` pod |
| 13 | `verify-no-eviction-in-progress-left` | Asserts no pod carries `descheduler.alpha.kubernetes.io/eviction-in-progress` |

**Cleanup:** Step 6 cleanup deletes the Descheduler CR. The test namespace (with the StatefulSet and pods) is auto-deleted by Chainsaw.

## Files

| File | Purpose |
|------|---------|
| `manifests/sts-pinned-annotated.yaml` | StatefulSet placed on `($targetNode)` via `nodeName`, pod template annotated with `request-evict-only` |
| `manifests/descheduler-cr.yaml` | Descheduler CR with `removeDuplicates`, scoped by `podLabelSelector` |
| `../common/manifests/sts-unpin-patch.yaml` | Shared patch clearing the template `nodeName` |
| `../common/asserts/assert-descheduler-ready.yaml` | Shared assert: the deployment has a ready replica |
| `../common/asserts/assert-descheduler-rollout-complete.yaml` | Shared assert: rollout finished and the pod runs the current policy |

## How Pods Are Concentrated Without Cordoning

Same mechanism as `../statefulset-remove-duplicates/`:

- The template sets `spec.nodeName: <targetNode>`, so the **kubelet admits the pods directly, bypassing the scheduler** — all 3 land on the target node.
- `updateStrategy: OnDelete` guarantees a later template patch never restarts the already-running pods.
- Step 7 clears `nodeName` in the template. Running pods stay put, but their live specs carry no node-scheduling constraints, so the descheduler's hardcoded `nodeFit: true` considers them evictable, and recreated pods go through the scheduler.
- A required self-`podAntiAffinity` makes recreated pods deterministically land on a different node, so redistribution does not depend on scheduler scoring luck.

The only difference from `statefulset-remove-duplicates` is the `descheduler.alpha.kubernetes.io/request-evict-only` annotation on the pod template — which is exactly the variable this test isolates.

## Running

```bash
# From the suite root
cd modules/400-descheduler/e2e/descheduler
task evictions-in-background:run

# Or from this directory
task run
task dry-run
```

## Pass/Fail Criteria

- **Pass:** the gate is in the Deployment and in the running pod, the descheduler is ready, the 3 annotated pods end up on ≥2 nodes with a `RemoveDuplicates` event, and no pod is left marked as an eviction in progress.
- **Fail:** the gate is missing (module regression), the descheduler pod is not ready (the gate was rejected by the binary — likely renamed upstream), fewer than 2 eligible worker nodes, CR create denied (RBAC), policy not rendered, or the annotated pods stay on a single node.

## Troubleshooting

### Descheduler pod is not ready after the bump

An upstream bump may have graduated, renamed or removed the gate:

```bash
kubectl -n d8-descheduler logs -l app=descheduler -c descheduler --tail=50 | grep -i "feature"
```

`unrecognized feature gate: EvictionsInBackground` means the module must stop passing it (the gate went GA and was retired) or pass the new name.

### Annotated pods stay on one node (0 evictions)

```bash
kubectl -n d8-descheduler logs -l app=descheduler -c descheduler | grep -iE "RemoveDuplicates|background|feasible|skipping"
kubectl -n <test-namespace> get pods -o wide
```

`Eviction in background assumed` in the log would mean something in the cluster *is* answering the eviction request with `429` + `Eviction triggered evacuation` — the test's premise no longer holds in that cluster.

A more common cause is fewer than 2 schedulable worker nodes (RemoveDuplicates skips the owner).
