# capi-crd-migration

**Name:** `capi-crd-migration`
**Package:** `internal/controller/crdmigration`
**Primary resource:** `CustomResourceDefinition`

## Purpose

Migrates the cluster-api CRDs to `v1beta2` storage and rewires conversion webhooks so the
rest of the CAPI controllers (see `controller-capi.md`) can create v1beta2 objects.

This is the unblocking step: helm cannot apply v1beta2 manifests until the CRDs actually serve
and store `v1beta2`. The reconciler applies the embedded CRD manifests (from `/crds`),
switches the storage version, and points the conversion webhook at the in-cluster service —
without waiting for `capi-controller-manager`, which may itself be failing precisely because
the CRDs are not yet served in v1beta2.

It manages two CRD groups:

| Group | CRDs | Conversion webhook target |
|-------|------|---------------------------|
| `capiCRDNames` (cluster.x-k8s.io) | clusters, machines, machinesets, machinedeployments, machinehealthchecks, machinedrainrules, machinepools, extensionconfigs | `capi-webhook-service` (CA from `capi-webhook-tls`) |
| `conversionCRDNames` (deckhouse.io) | nodegroups, instances | `node-controller-webhook` (CA from `node-controller-webhook-tls`) |

## Watched Resources

| Resource | Trigger | MapFunc |
|----------|---------|---------|
| `CustomResourceDefinition` | Any change (primary) | — |
| startup source | Controller start | Enqueue all `capiCRDNames` + `conversionCRDNames` (informer is empty on fresh install) |
| `Secret` `capi-webhook-tls` | CA rotation | Enqueue all CAPI CRDs |
| `Secret` `node-controller-webhook-tls` | CA rotation | Enqueue conversion CRDs (nodegroups, instances) |

The embedded CRD manifests are loaded once in `Setup` via `loadCRDs("/crds")` and cached in
`r.crdSpecs`.

## Reconciliation Logic

```
CRD request (or Secret re-enqueue)
  │
  ├─ isConversionCRD(name)? (nodegroups/instances)
  │   └─ reconcileConversionWebhook:
  │       ├─ read node-controller-webhook-tls → ca.crt (requeue 30s if missing/empty)
  │       └─ patchConversionWebhook → point /convert at node-controller-webhook
  │
  ├─ not in capiCRDNames? → done (ignored)
  │
  ├─ no embedded manifest for this CRD? → done
  │
  ├─ checkPreconditions: read capi-webhook-tls → ca.crt
  │   └─ missing/empty? → requeue 30s (do NOT wait for capi-controller-manager)
  │
  ├─ CRD not found (fresh install):
  │   ├─ create from embedded spec + setMigrationSpec(caBundle)
  │   └─ AlreadyExists race → requeue 5s
  │
  └─ CRD exists:
      └─ full apply: replace spec with embedded spec, setMigrationSpec, MergeFrom patch
```

### setMigrationSpec

- **Storage switch:** only when `v1beta2` is present in `spec.versions`, mark `v1beta2` as the
  single `storage: true` version (all others `false`).
- **Conversion webhook:** strategy `Webhook`, service `capi-webhook-service`/`d8-cloud-instance-manager`
  path `/convert` port 443, CA bundle from `capi-webhook-tls`,
  `conversionReviewVersions: [v1, v1beta1]`.

### Conversion webhooks (deckhouse.io CRDs)

`patchConversionWebhook` points `nodegroups`/`instances` conversion at the
`node-controller-webhook` service (`conversionReviewVersions: [v1]`), and is idempotent via
`isConversionWebhookCurrent` (skips when service/namespace/CA already match).

## One-shot path: `EnsureCRDs`

`ensure.go` exposes `EnsureCRDs(ctx, client)` — a synchronous bootstrap variant invoked outside
the reconcile loop. It:

1. Patches conversion webhooks on deckhouse CRDs **first** (only if a multi-version conversion
   CRD already exists), to unblock the API server before touching CAPI CRDs.
2. Reads the `capi-webhook-tls` CA **best-effort** via `bestEffortCABundle` (single Get, no
   blocking). On a fresh static cluster `capi-webhook-tls` is not rendered until a NodeGroup
   with `staticInstances` appears (capi is disabled before that), so a hard wait would deadlock
   startup. If the secret is absent the CA is `nil` and CRDs are created without a CABundle (the
   field is optional); the reconciler fills it later via the `checkPreconditions` 30s requeue.
3. Applies every embedded CAPI CRD through `ensureSingleCRD` (create-or-full-apply, same
   `setMigrationSpec` logic as the reconciler).

## Files

- `controller.go` — reconciler, watches (startup source + Secret), `setMigrationSpec`, `loadCRDs`
- `ensure.go` — synchronous `EnsureCRDs` bootstrap path, conversion-webhook patching, best-effort CA read
