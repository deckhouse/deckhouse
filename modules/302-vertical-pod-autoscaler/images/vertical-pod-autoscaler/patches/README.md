# Patches

## 002-openkruise-daemonset-apiversion.patch

This patch for correction work with VPA in Deckhouse with OpenKruise DaemonSet (apiVersion == apps.kruise.io/v1alpha1)

## 003-recommender.patch

This patch is not working for prometheus storage. Only for VPA checkpoints.
Have no idea, what it is for.
As we use Prometheus storage, will not move this patch.

## 004-daemonset-scope-node-label.patch

Adds DaemonSet scoped recommendations grouped by node label key from `spec.scope`.

- Supports only DaemonSet targetRef with non-empty `spec.scope`.
- Uses node label value as a recommendation group key for Prometheus-based flow.
- Uses `status.groups` as source-of-truth for scoped recommendations.
- Stores grouped recommendations in compact form (target-focused payload).
- Keeps `status.recommendation` as non-scoped summary to avoid duplication.
- Uses map-based lookup in admission-controller/updater for scoped recommendation selection.

