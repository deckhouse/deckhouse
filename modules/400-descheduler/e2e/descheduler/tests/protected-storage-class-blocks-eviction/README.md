# Descheduler — protected StorageClass blocks eviction

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates `spec.protectedStorageClasses`: a pod using a PersistentVolumeClaim from a listed StorageClass is never evicted, while the same pod is evicted once the StorageClass is removed from the list.

**What it does:** Creates a single-replica StatefulSet with a `volumeClaimTemplate` backed by the cluster's default StorageClass, then force-places a conflict pod on the same node so the StatefulSet pod violates its own required `podAntiAffinity` — the same deterministic eviction trigger the single-replica test uses. With `protectedStorageClasses` set, a full descheduling cycle must leave the pod untouched. The CR is then updated to drop the protection, and the very same pod must be evicted.

## Prerequisites

- Multi-node Kubernetes cluster (at least **2** schedulable worker nodes)
- Exactly one **default StorageClass** able to provision a 1Gi `ReadWriteOnce` volume
- Descheduler pre-installed in the `d8-descheduler` namespace
- Deckhouse **ClusterAdmin**-level rights to create `Descheduler` CRs (a plain `kubernetes-admin` identity is denied)
- Chainsaw CLI installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-module-installed` | Asserts the `deschedulers.deckhouse.io` CRD exists |
| 2 | `check-minimum-nodes` | Verifies ≥2 eligible worker nodes (so `nodeFit` would allow eviction and only the protection stands in the way) |
| 3 | `check-default-storage-class` | Verifies exactly one default StorageClass exists and binds it to `$storageClass` |
| 4 | `create-statefulset` | Creates a 1-replica StatefulSet with a `volumeClaimTemplate` on `$storageClass` |
| 5 | `wait-statefulset-ready` | Waits until the pod is Running, the PVC is `Bound` and the STS is 1/1 ready |
| 6 | `create-conflict-pod` | Captures the pod's node, force-places `conflict-pod` there via `spec.nodeName`, making the StatefulSet pod violate its anti-affinity |
| 7 | `verify-protected-pod-is-not-evicted` | Applies the CR with `protectedStorageClasses`, asserts the rendered policy carries `podProtections`/`PodsWithPVC`/the StorageClass name, waits for the rollout, captures the pod UID, sleeps one cycle (300s), then asserts: no eviction event, identical UID, pod unmoved |
| 8 | `verify-pod-is-evicted-without-protection` | Captures the UID, updates the same CR without `protectedStorageClasses`, asserts the policy lost `PodsWithPVC`, waits for the rollout, then asserts the pod **was** evicted (new UID) and an eviction event exists |

**Cleanup:** Step 8 cleanup deletes the Descheduler CR. The test namespace (StatefulSet, PVC, conflict pod) is auto-deleted by Chainsaw.

## Files

| File | Purpose |
|------|---------|
| `manifests/sts-with-pvc.yaml` | 1-replica StatefulSet with a `volumeClaimTemplate` and a required `podAntiAffinity` against the conflict label |
| `manifests/conflict-pod.yaml` | Bare pod carrying the repelled label, force-placed via `spec.nodeName` |
| `manifests/descheduler-cr-protected.yaml` | Descheduler CR with `protectedStorageClasses` and `removePodsViolatingInterPodAntiAffinity` |
| `manifests/descheduler-cr-unprotected.yaml` | Same CR without `protectedStorageClasses` (the control) |
| `../common/asserts/assert-descheduler-rollout-complete.yaml` | Shared assert: rollout finished and the pod runs the current policy |

## Why This Is a Negative Test (and how it stays honest)

1. **The pod is genuinely evictable.** The eviction trigger is a real, required anti-affinity violation, the live pod spec carries no node constraint (it is placed by the scheduler, never pinned via `nodeName` or `nodeSelector`), and ≥2 worker nodes are asserted — so `nodeFit` would allow the eviction.
2. **The new policy is proven live before checking.** `assert-descheduler-rollout-complete.yaml` asserts the running descheduler pod carries the current `checksum/config`, so a full cycle really ran against the new policy during the 300s sleep.
3. **A control phase proves causality.** Step 8 drops only `protectedStorageClasses` from the same CR, against the same pod and the same violation, and requires the eviction to happen. Without it, step 7 could pass for any unrelated reason.
4. **The rendered policy is asserted, not just the outcome.** The `PodsWithPVC` key under `podProtections.config` is matched by name upstream and an unknown key is silently ignored, which would protect *every* pod with a PVC instead of the listed StorageClasses. Step 7 asserts the ConfigMap actually contains `podProtections`, `PodsWithPVC` and the StorageClass name.

## Policy Config

- `protectedStorageClasses: [<default StorageClass>]` renders as `podProtections.extraEnabled: ["PodsWithPVC"]` plus `podProtections.config.PodsWithPVC.protectedStorageClasses`.
- In that branch the deprecated `evictLocalStoragePods` / `evictSystemCriticalPods` / `ignorePvcPods` / `evictFailedBarePods` flags are not rendered at all: upstream `DefaultEvictor` validation rejects a policy that mixes them with `podProtections`, and the descheduler would refuse to start.

## Running

```bash
# From the e2e directory
task protected-storage-class-blocks-eviction:run

# Or directly
chainsaw test --test-dir ./tests/protected-storage-class-blocks-eviction/
```

This test runs two descheduling phases (~10–12 min) because of the deliberate 300s sleep plus two descheduler rollouts.

## Pass/Fail Criteria

- **Pass:** with the protection in place the pod keeps its UID and node and no eviction event appears; after the protection is dropped the pod is recreated with a new UID and an eviction event is recorded.
- **Fail:** the protected pod is evicted, the control phase does not evict (the test would otherwise be vacuous), or the rendered policy does not carry `podProtections`/`PodsWithPVC`.

## Troubleshooting

### The protected pod WAS evicted

Check that the rendered policy really carries the protection and that the PVC resolves to the expected StorageClass:

```bash
kubectl -n d8-descheduler get cm descheduler-policy -o jsonpath='{.data.policy\.yaml}'
kubectl -n <test-namespace> get pvc data-test-sts-pvc-0 -o jsonpath='{.spec.storageClassName}'
```

### The control phase does not evict

The pod is protected by something else than the StorageClass — most often `nodeFit` (no second node accepts the pod, which happens when the volume is node-local and the replacement can only return to the same node). Check the descheduler log:

```bash
kubectl -n d8-descheduler logs -l app=descheduler -c descheduler | grep -iE "nodeFit|does not fit|protected"
```

### `create deschedulers ... is forbidden`

```bash
kubectl auth can-i create deschedulers.deckhouse.io
```

Run under a Deckhouse ClusterAdmin-level identity (see `../../README.md`).
