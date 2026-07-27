# Descheduler StatefulSet — single-replica eviction

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that proves the descheduler will evict **even the only pod** of a single-replica StatefulSet, because Deckhouse does not expose the upstream `minReplicas` protection.

**What it does:** Creates a single-replica StatefulSet on a target node, force-places a bare "conflict" pod next to it that the StatefulSet pod's required anti-affinity repels, then enables `removePodsViolatingInterPodAntiAffinity`. The descheduler evicts `test-sts-0` (the only replica); the scheduler honors the anti-affinity and recreates it on a different node.

> **Note on the trigger:** an aggressive `lowNodeUtilization` setup would also evict the pod, but it depends on live node utilization and is flaky in e2e. This test uses a deterministic inter-pod anti-affinity violation instead, with the same conclusion: the single replica is evicted and nothing protects it.

## Prerequisites

- Multi-node Kubernetes cluster (at least **2** schedulable worker nodes — so the evicted pod can be rescheduled elsewhere)
- Descheduler pre-installed in the `d8-descheduler` namespace
- Deckhouse **ClusterAdmin**-level rights to create `Descheduler` CRs (a plain `kubernetes-admin` identity is denied)
- Chainsaw CLI installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-module-installed` | Asserts the `deschedulers.deckhouse.io` CRD exists |
| 2 | `check-minimum-nodes` | Verifies ≥2 Ready, schedulable, non-control-plane worker nodes |
| 3 | `create-statefulset` | Creates a 1-replica StatefulSet pinned to the target node (its pod carries a required anti-affinity vs the conflict label) |
| 4 | `wait-statefulset-ready` | Waits until `test-sts-0` is Running on the target node and the STS is 1/1 ready |
| 5 | `release-node-pinning` | Patches the template to clear `nodeName` so the replacement is scheduled normally; asserts the pod stays put |
| 6 | `create-conflict-pod` | Force-places a bare labeled pod on the same node via `spec.nodeName`, making `test-sts-0` violate its anti-affinity |
| 7 | `trigger-eviction-and-verify` | Captures `test-sts-0`'s UID, applies the CR, waits for the rollout, then asserts the pod was evicted (new UID, different node), a `RemovePodsViolatingInterPodAntiAffinity` event exists, and the STS is 1/1 again (cleanup deletes the CR) |

**Cleanup:** Step 7 cleanup deletes the Descheduler CR. The test namespace (with the StatefulSet and conflict pod) is auto-deleted by Chainsaw.

> Step 7 bundles capture → apply → verify into a single chainsaw step on purpose: operation `outputs` (the captured initial pod UID) are not visible across step boundaries.

## Files

| File | Purpose |
|------|---------|
| `../common/sts-pinned.yaml` | Shared StatefulSet template placed on `($targetNode)` via `nodeName` |
| `../common/sts-unpin-patch.yaml` | Shared patch clearing the template `nodeName` |
| `../common/assert-descheduler-rollout-complete.yaml` | Shared assert: rollout finished and the pod runs the current policy |
| `manifests/conflict-pod.yaml` | Bare labeled pod creating the anti-affinity violation |
| `manifests/descheduler-cr.yaml` | Descheduler CR with `removePodsViolatingInterPodAntiAffinity` |

## Why the Anti-Affinity Lives on the StatefulSet Pod

Upstream `RemovePodsViolatingInterPodAntiAffinity` evicts the pod that **carries** a violated required anti-affinity rule — not the pod it points at. So the rule must be on `test-sts-0` (via the shared template), and the bare `conflict-pod` merely carries the repelled label:

- `conflict-pod` is force-placed on the target node with `spec.nodeName` (kubelet admission does not enforce inter-pod anti-affinity, so co-location succeeds).
- Now `test-sts-0` violates its own required rule → it becomes the deterministic eviction candidate.
- The conflict pod can **never** be evicted: it has no owner reference (a Running bare pod is filtered by `DefaultEvictor`) and does not match the CR `podLabelSelector`.
- The eviction is proven by `test-sts-0`'s **UID change** + **node change** + a `RemovePodsViolatingInterPodAntiAffinity` event (the event reason is the strategy name; `Descheduled` is the event *action*).

## Policy Config

- `removePodsViolatingInterPodAntiAffinity.enabled: true`, scoped by `podLabelSelector` to `e2e-test: sts-single-replica`.
- No `minReplicas` — Deckhouse's CRD has no such field (see the `descheduler-minreplicas-not-supported` test), which is exactly why the lone replica is evictable.

## Running

```bash
# From the e2e directory
task run:statefulset-single-replica-eviction

# Or directly
chainsaw test --test-dir ./tests/statefulset-single-replica-eviction/
```

## Pass/Fail Criteria

- **Pass:** `test-sts-0` is recreated with a new UID on a different node, a `RemovePodsViolatingInterPodAntiAffinity` event is recorded for it, and the STS returns to 1/1 ready.
- **Fail:** the pod is never evicted (same UID/node), no event, or the rollout/CR steps fail.

## Troubleshooting

### The pod is not evicted

The most common cause is the anti-affinity not actually being violated, or the wrong pod being the candidate:

```bash
kubectl -n <test-namespace> get pods -o wide          # conflict-pod and test-sts-0 on the same node?
kubectl -n d8-descheduler logs -l app=descheduler -c descheduler | grep -iE "InterPodAntiAffinity|evict"
```

Both pods must be co-located on the target node for `test-sts-0` to violate its rule.

### `create deschedulers ... is forbidden`

```bash
kubectl auth can-i create deschedulers.deckhouse.io
```

Run under a Deckhouse ClusterAdmin-level identity (see `../../README.md`).
