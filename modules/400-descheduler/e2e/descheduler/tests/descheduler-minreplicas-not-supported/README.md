# Descheduler — minReplicas not supported

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that proves the upstream descheduler `DefaultEvictor.minReplicas` protection (do not evict pods of an owner with fewer than N replicas) is **not exposed by Deckhouse and cannot be smuggled in**.

**What it does:** First tries to set `spec.minReplicas` on a `Descheduler` CR and confirms it never persists. Then it manually injects a `minReplicas` marker into the rendered `descheduler-policy` ConfigMap and confirms Deckhouse overwrites it on the next render.

> This test does not involve a StatefulSet itself — it validates the module's CRD schema and ConfigMap reconciliation. It is grouped with the StatefulSet tests because `minReplicas` is the upstream knob that would otherwise protect a single-replica StatefulSet (see the `statefulset-single-replica-eviction` test).

## Prerequisites

- Kubernetes cluster with the descheduler module installed in `d8-descheduler`
- Deckhouse **ClusterAdmin**-level rights to create `Descheduler` CRs **and** patch the `descheduler-policy` ConfigMap in `d8-descheduler`
- Chainsaw CLI installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name | Description |
|------|------|-------------|
| 1 | `assert-module-installed` | Asserts the `deschedulers.deckhouse.io` CRD exists |
| 2 | `attempt-minreplicas-cr` | Applies a CR with `spec.minReplicas: 2` (tolerating either reject or prune via `expect`) and asserts **no** Descheduler in the cluster carries `spec.minReplicas` |
| 3 | `apply-probe-cr` | Applies a valid inert probe CR so Deckhouse renders the policy ConfigMap; asserts it contains the `e2e-minreplicas-probe` profile (cleanup deletes the probe CR) |
| 4 | `inject-minreplicas-into-configmap` | Patches the ConfigMap to append a `minReplicas` marker line to `policy.yaml` (computed via `x_k8s_get`); asserts the marker is present |
| 5 | `trigger-module-rerender` | Applies an updated probe CR (changed `podLabelSelector`) to force Deckhouse to re-render the module |
| 6 | `verify-manual-edit-overwritten` | Asserts the re-rendered `policy.yaml` contains the updated profile **and no `minReplicas`** |

**Cleanup:** Step 3 cleanup deletes the probe CR (Deckhouse then re-renders the ConfigMap without it). The test creates no namespaced workloads.

## Files

| File | Purpose |
|------|---------|
| `manifests/descheduler-cr-minreplicas.yaml` | Invalid CR with `spec.minReplicas: 2` |
| `manifests/descheduler-cr.yaml` | Valid inert probe CR (its `podLabelSelector` matches no pods) |
| `manifests/descheduler-cr-updated.yaml` | Probe CR variant with a changed selector that forces a re-render |

## The CRD has no `minReplicas` field

`spec.minReplicas` is not part of the `Descheduler` v1alpha2 schema. Depending on the API server field-validation mode the apply is either **rejected** (`Strict`, `kubectl apply`'s default since 1.27) or the unknown field is silently **pruned** (`Warn`, the default for direct API clients such as chainsaw). The `apply` step tolerates both via `expect`, and the test then asserts the invariant that holds either way: **no Descheduler resource in the cluster carries `spec.minReplicas`**.

## A manual ConfigMap edit is overwritten

Even if an operator edits the rendered policy by hand, Deckhouse owns the ConfigMap and re-renders it from the CRs:

1. A probe CR makes Deckhouse render `descheduler-policy`.
2. The test appends a `minReplicas` marker to `policy.yaml` (a no-bash patch built with the `x_k8s_get` function) and confirms it is present.
3. Updating the probe CR forces an immediate re-render (deterministic, instead of waiting for an unspecified drift-reconciliation cycle).
4. The re-rendered policy contains the updated profile but **no `minReplicas`** — the manual edit is gone.

## Running

```bash
# From the e2e directory
task run:minreplicas-not-supported

# Or directly
chainsaw test --test-dir ./tests/descheduler-minreplicas-not-supported/
```

## Pass/Fail Criteria

- **Pass:** no Descheduler CR ends up with `spec.minReplicas`, and after the re-render the policy ConfigMap shows the updated profile without `minReplicas`.
- **Fail:** a CR persists `minReplicas`, the ConfigMap is not rendered/re-rendered, or the manual `minReplicas` marker survives the re-render.

## Troubleshooting

### The manual marker survives (step 6 times out)

The re-render may not have happened yet, or the runner cannot patch the ConfigMap:

```bash
kubectl -n d8-descheduler get configmap descheduler-policy -o jsonpath='{.data.policy\.yaml}' | grep -n minReplicas
kubectl get deschedulers.deckhouse.io
kubectl auth can-i patch configmaps -n d8-descheduler
```

Deckhouse re-renders within ~1 minute of a CR change; the test allows up to 300s.

### `create deschedulers ... is forbidden`

```bash
kubectl auth can-i create deschedulers.deckhouse.io
```

Run under a Deckhouse ClusterAdmin-level identity (see `../../README.md`).
