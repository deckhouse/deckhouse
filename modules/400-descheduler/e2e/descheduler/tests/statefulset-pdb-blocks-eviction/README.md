# Descheduler StatefulSet — PDB blocks eviction (maxUnavailable=0)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates a `PodDisruptionBudget` with `maxUnavailable: 0` blocks **every** descheduler eviction.

**What it does:** Creates a 3-replica StatefulSet stacked on one node (same technique as the RemoveDuplicates test), adds a PDB that forbids any voluntary disruption, then enables `removeDuplicates`. The descheduler detects duplicates and attempts eviction, but the API server rejects each request (HTTP 429). The test waits a full descheduling cycle and proves nothing changed.

## Prerequisites

- Multi-node Kubernetes cluster (at least **2** schedulable worker nodes)
- Descheduler pre-installed in the `d8-descheduler` namespace
- Deckhouse **ClusterAdmin**-level rights to create `Descheduler` CRs (a plain `kubernetes-admin` identity is denied)
- Chainsaw CLI installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-module-installed` | Asserts the `deschedulers.deckhouse.io` CRD exists |
| 2 | `check-minimum-nodes` | Verifies ≥2 eligible worker nodes (so pods are evictable by `nodeFit` and only the PDB stands in the way) |
| 3 | `create-statefulset` | Creates a 3-replica StatefulSet pinned to one node via `spec.nodeName` |
| 4 | `wait-statefulset-pinned` | Waits until all 3 pods are Running on the target node and the STS is 3/3 ready |
| 5 | `release-node-pinning` | Patches the template to clear `nodeName` (OnDelete keeps running pods); asserts pods stay put |
| 6 | `create-pdb` | Creates a PDB `maxUnavailable: 0` and asserts `expectedPods: 3`, `disruptionsAllowed: 0` |
| 7 | `apply-descheduler-cr` | Applies the `removeDuplicates` CR scoped by `podLabelSelector` (cleanup deletes it) |
| 8 | `assert-policy-rendered` | Asserts the policy ConfigMap contains the `e2e-sts-pdb-blocks` profile |
| 9 | `wait-descheduler-rollout` | Waits for the rollout to finish with the new policy checksum (else the negative checks could pass vacuously) |
| 10 | `verify-eviction-blocked` | Captures pod UIDs, sleeps one cycle (300s), then asserts: no `RemoveDuplicates` event, identical UIDs, pods unmoved, STS 3/3, PDB still `disruptionsAllowed: 0` |

**Cleanup:** Step 7 cleanup deletes the Descheduler CR. The test namespace (with the StatefulSet and PDB) is auto-deleted by Chainsaw.

## Files

| File | Purpose |
|------|---------|
| `../common/sts-pinned.yaml` | Shared StatefulSet template placed on `($targetNode)` via `nodeName` |
| `../common/sts-unpin-patch.yaml` | Shared patch clearing the template `nodeName` |
| `../common/assert-descheduler-rollout-complete.yaml` | Shared assert: rollout finished and the pod runs the current policy |
| `manifests/pdb.yaml` | PDB with `maxUnavailable: 0` selecting the test pods |
| `manifests/descheduler-cr.yaml` | Descheduler CR with `removeDuplicates`, scoped by `podLabelSelector` |

## Why This Is a Negative Test (and how it stays honest)

The test must prove an **absence** of action, which is easy to get wrong. Three safeguards keep it meaningful:

1. **Real eviction is actually attempted.** The setup is identical to the passing RemoveDuplicates test (≥2 nodes, no node constraints in the live pod specs), so `nodeFit` would allow eviction — only the PDB blocks it. The node gate prevents a false pass where eviction is skipped for the wrong reason.
2. **The new policy is proven live before checking.** A bare `readyReplicas >= 1` would be satisfied by the old descheduler pod during a rolling update; instead `wait-descheduler-rollout` asserts the running pod carries the current `checksum/config`, so a full descheduling cycle really ran against the new policy during the 300s sleep.
3. **The "nothing happened" check is multi-signal.** Pod **UIDs** are captured before the cycle (via operation `outputs`) and compared after — a stronger signal than event absence. Combined with: no `RemoveDuplicates` event, pods unmoved on the target node, STS 3/3, and PDB still `disruptionsAllowed: 0`.

A failed eviction emits no success event in this module's configuration, so the `error` operation checks specifically for the `RemoveDuplicates` (success) reason.

## Policy Config

- `removeDuplicates.enabled: true`, scoped by `podLabelSelector` to `e2e-test: sts-pdb-blocks`.
- PDB `maxUnavailable: 0` → the Eviction API rejects every voluntary disruption with HTTP 429.

## Running

```bash
# From the e2e directory
task run:statefulset-pdb-blocks-eviction

# Or directly
chainsaw test --test-dir ./tests/statefulset-pdb-blocks-eviction/
```

This test is the slowest of the suite (~5–6 min) because of the deliberate 300s sleep.

## Pass/Fail Criteria

- **Pass:** after a full cycle, no `RemoveDuplicates` event, identical pod UIDs, all 3 pods still Running on the target node, STS 3/3, PDB still `disruptionsAllowed: 0`.
- **Fail:** any pod recreated/moved, a `RemoveDuplicates` event appears, PDB not at 0, or the rollout/CR steps fail.

## Troubleshooting

### A pod WAS evicted (test fails on UID/move check)

Confirm the PDB was actually in force during the cycle:

```bash
kubectl -n <test-namespace> get pdb test-sts-pdb -o wide
kubectl -n d8-descheduler logs -l app=descheduler -c descheduler | grep -iE "429|disruption|cannot evict"
```

If `disruptionsAllowed` was not 0 (e.g. the selector didn't match the pods), eviction would succeed — check the PDB `selector` vs the pod labels.

### `create deschedulers ... is forbidden`

```bash
kubectl auth can-i create deschedulers.deckhouse.io
```

Run under a Deckhouse ClusterAdmin-level identity (see `../../README.md`).
