# Descheduler StatefulSet — RemoveDuplicates (no PDB)

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates the **RemoveDuplicates** strategy evicts StatefulSet pods that are stacked on a single node so they spread across the cluster.

**What it does:** Creates a 3-replica StatefulSet concentrated on one worker node (via `spec.nodeName`, no cordoning), then applies a `removeDuplicates` Descheduler CR scoped to the test pods. The descheduler treats the 3 same-owner pods on one node as duplicates and evicts the excess; the StatefulSet controller recreates them and the scheduler places them on other nodes.

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
| 3 | `create-statefulset` | Creates a 3-replica StatefulSet pinned to one node via `spec.nodeName` |
| 4 | `wait-statefulset-pinned` | Waits until `test-sts-0/1/2` are Running on the target node and the STS is 3/3 ready |
| 5 | `release-node-pinning` | Patches the template to clear `nodeName` (OnDelete keeps running pods); asserts all 3 pods stay put |
| 6 | `apply-descheduler-cr` | Applies the `removeDuplicates` Descheduler CR scoped by `podLabelSelector` (cleanup deletes it) |
| 7 | `assert-policy-rendered` | Asserts the `descheduler-policy` ConfigMap contains the `e2e-sts-remove-duplicates` profile |
| 8 | `wait-descheduler-rollout` | Waits for the descheduler rollout to finish and the running pod to carry the new policy checksum |
| 9 | `verify-pods-redistributed` | Asserts the 3 pods spread across ≥2 nodes and the STS returns to 3/3 ready |
| 10 | `verify-eviction-events` | Asserts a `RemoveDuplicates` eviction event exists for a `test-sts-*` pod |

**Cleanup:** Step 6 cleanup deletes the Descheduler CR. The test namespace (with the StatefulSet and pods) is auto-deleted by Chainsaw.

## Files

| File | Purpose |
|------|---------|
| `../common/sts-pinned.yaml` | Shared StatefulSet template placed on `($targetNode)` via `nodeName` |
| `../common/sts-unpin-patch.yaml` | Shared patch clearing the template `nodeName` |
| `../common/assert-descheduler-rollout-complete.yaml` | Shared assert: rollout finished and the pod runs the current policy |
| `manifests/descheduler-cr.yaml` | Descheduler CR with `removeDuplicates`, scoped by `podLabelSelector` |

## How Pods Are Concentrated Without Cordoning

The classic way to stack pods on one node is `kubectl cordon`. This test avoids node mutations and bash entirely:

- The StatefulSet template sets `spec.nodeName: <targetNode>`, so the **kubelet admits the pods directly, bypassing the scheduler** — all 3 land on the target node.
- `updateStrategy: OnDelete` guarantees a later template patch never restarts the already-running pods.
- After the pods are Running, step 5 patches the template to **clear `nodeName`**. Running pods stay (OnDelete), but their live specs now carry **no node-scheduling constraints**, so the descheduler's hardcoded `nodeFit: true` considers them evictable, and any pod recreated after an eviction goes through the scheduler.
- The template also carries a required **self-`podAntiAffinity`** (repels its own `e2e-test` label). It is inert at creation (kubelet ignores it for `nodeName`-placed pods) but is enforced for recreated pods, so they deterministically land on a different node — redistribution does not depend on scheduler scoring luck.

A required node **affinity** would NOT work here: it would stay in the live pod spec forever and make `nodeFit` reject every eviction.

## Policy Config

- `removeDuplicates.enabled: true`
- `podLabelSelector: { matchLabels: { e2e-test: sts-remove-duplicates } }` — eviction is scoped to this test's pods only, so the test is safe to run on a cluster with other workloads.

### How the descheduler decides

`RemoveDuplicates` ensures no more than one pod of the same owner (here the StatefulSet) runs on a node. With 3 pods on one node and N≥2 feasible target nodes it evicts pods above the per-node average (`ceil(3/N)`), leaving e.g. a 2+1 spread on 2 nodes. It skips the owner entirely if it sees fewer than 2 feasible target nodes — hence the ≥2-node precondition.

### Eviction events

The eviction **event reason is the strategy name** (`RemoveDuplicates`); `Descheduled` is the event *action*, not the reason. An assertion on `reason: Descheduled` would never match in descheduler v0.35.x.

## Running

```bash
# From the e2e directory
task run:statefulset-remove-duplicates

# Or directly
chainsaw test --test-dir ./tests/statefulset-remove-duplicates/
```

## Pass/Fail Criteria

- **Pass:** pods end up on ≥2 nodes, the StatefulSet is 3/3 ready, and a `RemoveDuplicates` event exists for a `test-sts-*` pod.
- **Fail:** fewer than 2 eligible worker nodes, descheduler/CRD missing, CR create denied (RBAC), policy not rendered, or pods stay on a single node.

## Troubleshooting

### Pods stay on one node (0 evictions)

Check that the descheduler ran the strategy and why it skipped:

```bash
kubectl -n d8-descheduler logs -l app=descheduler -c descheduler | grep -iE "RemoveDuplicates|feasible|skipping"
kubectl -n <test-namespace> get pods -o wide
```

A common cause is fewer than 2 schedulable worker nodes (RemoveDuplicates skips the owner).

### `create deschedulers ... is forbidden`

The runner identity lacks rights to create `Descheduler` CRs:

```bash
kubectl auth can-i create deschedulers.deckhouse.io
```

Run the suite under a Deckhouse ClusterAdmin-level identity (see `../../README.md`).

### Rollout never completes

The policy change restarts the descheduler. Inspect the rollout:

```bash
kubectl -n d8-descheduler rollout status deployment/descheduler
kubectl -n d8-descheduler get pods -l app=descheduler
```
