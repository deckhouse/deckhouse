# Descheduler StatefulSet — PDB allows one disruption (maxUnavailable=1)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates a `PodDisruptionBudget` with `maxUnavailable: 1` lets the descheduler rebalance a StatefulSet **one pod at a time** without ever dropping below quorum.

**What it does:** Creates a 3-replica StatefulSet stacked on one node (same technique as the RemoveDuplicates test), adds a PDB that allows exactly one disruption, then enables `removeDuplicates`. The API server serializes the evictions (a second is rejected until the first replacement is Ready), so the StatefulSet stays available and ends up spread across nodes.

## Prerequisites

- Multi-node Kubernetes cluster (at least **2** schedulable worker nodes)
- Descheduler pre-installed in the `d8-descheduler` namespace
- Deckhouse **ClusterAdmin**-level rights to create `Descheduler` CRs (a plain `kubernetes-admin` identity is denied)
- Chainsaw CLI installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-module-installed` | Asserts the `deschedulers.deckhouse.io` CRD exists |
| 2 | `check-minimum-nodes` | Verifies ≥2 Ready, schedulable, non-control-plane worker nodes |
| 3 | `create-statefulset` | Creates a 3-replica StatefulSet pinned to one node via `spec.nodeName` |
| 4 | `wait-statefulset-pinned` | Waits until all 3 pods are Running on the target node and the STS is 3/3 ready |
| 5 | `release-node-pinning` | Patches the template to clear `nodeName` (OnDelete keeps running pods); asserts pods stay put |
| 6 | `create-pdb` | Creates a PDB `maxUnavailable: 1` and asserts `expectedPods: 3`, `disruptionsAllowed: 1` |
| 7 | `apply-descheduler-cr` | Applies the `removeDuplicates` CR scoped by `podLabelSelector` (cleanup deletes it) |
| 8 | `assert-policy-rendered` | Asserts the policy ConfigMap contains the `e2e-sts-pdb-allows-one` profile |
| 9 | `wait-descheduler-rollout` | Waits for the rollout to finish with the new policy checksum |
| 10 | `verify-pods-redistributed` | Asserts pods spread across ≥2 nodes, STS 3/3 ready, and the PDB recovered `disruptionsAllowed: 1` |
| 11 | `verify-eviction-events` | Asserts a `RemoveDuplicates` eviction event exists for a `test-sts-*` pod |

**Cleanup:** Step 7 cleanup deletes the Descheduler CR. The test namespace (with the StatefulSet and PDB) is auto-deleted by Chainsaw.

## Files

| File | Purpose |
|------|---------|
| `../common/sts-pinned.yaml` | Shared StatefulSet template placed on `($targetNode)` via `nodeName` |
| `../common/sts-unpin-patch.yaml` | Shared patch clearing the template `nodeName` |
| `../common/assert-descheduler-rollout-complete.yaml` | Shared assert: rollout finished and the pod runs the current policy |
| `manifests/pdb.yaml` | PDB with `maxUnavailable: 1` selecting the test pods |
| `manifests/descheduler-cr.yaml` | Descheduler CR with `removeDuplicates`, scoped by `podLabelSelector` |

## How Pods Are Concentrated Without Cordoning

Same technique as the `statefulset-remove-duplicates` test: `spec.nodeName` in the template stacks the pods on one node (kubelet bypasses the scheduler), `updateStrategy: OnDelete` keeps them put when the template is later patched to clear `nodeName`, and a required self-`podAntiAffinity` makes recreated pods land on different nodes. See that test's doc for the full rationale.

## How the PDB Serializes Evictions

With `maxUnavailable: 1` the Eviction API permits a disruption only while all other pods are healthy. The descheduler evicts one duplicate; that pod is recreated and must become Ready before the API server allows the next eviction. So the StatefulSet is never short more than one replica at a time. The test does not need a sleep — the outcome assertions poll until:

- the pods run on **≥2 different nodes** (`min(nodeName) != max(nodeName)`);
- the StatefulSet is back to 3/3 ready;
- the PDB has recovered its full budget (`currentHealthy: 3`, `disruptionsAllowed: 1`) — proving the disruption was transient and bounded.

The eviction **event reason is the strategy name** (`RemoveDuplicates`); `Descheduled` is the event *action*, not the reason.

## Policy Config

- `removeDuplicates.enabled: true`, scoped by `podLabelSelector` to `e2e-test: sts-pdb-allows-one`.
- PDB `maxUnavailable: 1` → at most one voluntary disruption in flight.

## Running

```bash
# From the e2e directory
task run:statefulset-pdb-allows-one-disruption

# Or directly
chainsaw test --test-dir ./tests/statefulset-pdb-allows-one-disruption/
```

## Pass/Fail Criteria

- **Pass:** pods end up on ≥2 nodes, STS 3/3 ready, PDB back to `disruptionsAllowed: 1`, and a `RemoveDuplicates` event exists for a `test-sts-*` pod.
- **Fail:** pods stay on one node, the StatefulSet drops below quorum, the PDB does not recover, or the rollout/CR steps fail.

## Troubleshooting

### Pods do not redistribute

```bash
kubectl -n d8-descheduler logs -l app=descheduler -c descheduler | grep -iE "RemoveDuplicates|disruption|429"
kubectl -n <test-namespace> get pods -o wide
kubectl -n <test-namespace> get pdb test-sts-pdb -o wide
```

If `disruptionsAllowed` stays 0 the replacement pod never became Ready (e.g. no room on other nodes) — verify node capacity and that ≥2 nodes are schedulable.

### `create deschedulers ... is forbidden`

```bash
kubectl auth can-i create deschedulers.deckhouse.io
```

Run under a Deckhouse ClusterAdmin-level identity (see `../../README.md`).
