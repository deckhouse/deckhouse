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
- Uses special `scopeValue="__absent__"` when node does not have `spec.scope` label key.
- Uses `status.groups` as source-of-truth for scoped recommendations.
- Stores grouped recommendations in compact form (`containerName` + `target` only).
- Does not duplicate `scope` inside grouped `containerRecommendations`; consumers resolve scope from `spec.scope` + `group.scopeValue`.
- Keeps fields mutually exclusive:
  - `status.recommendation` is used only for regular VPA (including DaemonSet without `spec.scope`);
  - `status.groups` is used only for scoped DaemonSet and `status.recommendation` is not populated.
- Admission-controller/updater read only the active status field for the current VPA mode.

## 005-default-update-mode-in-place-or-recreate.patch

Lets the module make `InPlaceOrRecreate` the default update mode instead of upstream `Recreate`, as the module documentation states.

- The default comes from the `VPA_DEFAULT_UPDATE_MODE` environment variable. Only `InPlaceOrRecreate` changes it. Without the variable, or with any other value, the default stays `Recreate`, as in upstream.
- The module templates set the variable on the updater and the admission controller: `InPlaceOrRecreate` on Kubernetes 1.33 and later, where in-place resize is available, and `Recreate` otherwise.
- `GetUpdateMode` returns this default when `spec.updatePolicy.updateMode` is not set, and the admission webhook writes it into a VPA without `spec.updatePolicy`.
- The updater reads the mode through `GetUpdateMode` when it selects pods for in-place updates. Upstream dereferences `spec.updatePolicy.updateMode` there directly, so with the new default a VPA without `updatePolicy` would crash the updater.
- Explicitly set modes are not changed, including `Recreate` and the deprecated `Auto`. VPAs that already store a mode written by the webhook keep it.

## 006-recommendation-factor.patch

Lets a container resource policy scale the recommendation by a per-resource factor.

- Adds `recommendationFactor` to `ContainerResourcePolicy`: a per-resource multiplier with the keys `cpu` and `memory`, set as Quantity strings, for example `"1.2"`. A value must be greater than 0 and at most 10, with at most two decimal places. A missing key means 1, and the webhook rejects any other resource name.
- The recommender applies the multiplier in its estimator chain, after the safety margin and the confidence multipliers and before the pod minimum, rounding and policy capping (`minAllowed`, `maxAllowed`). It scales `target`, `lowerBound` and `upperBound` alike.
- `uncappedTarget` keeps the value computed without the multiplier (the pod minimum and rounding still apply to it). The quality metrics and `lastRecommendation` stay on `uncappedTarget`, so they do not see the multiplier.
- An invalid value that is already stored (possible only if the webhook was bypassed) is ignored by the recommender with a log line, and the factor of that resource stays 1.
- The webhook and the recommender share the parser `Parse` from `pkg/utils/recommendationfactor`. It checks the range and the precision on the exact Quantity, because `MilliValue()` rounds sub-milli values up and overflows `int64`; a magnitude pre-screen on the `float64` approximation first rejects extreme exponents such as `1e1000000000` at a cost that does not depend on the exponent. Every error it returns carries a short rendering of the rejected value (`ValueOf`), which is what error and log messages print.
- `isNoopContainerPolicy` from patch 004 treats a container policy with `recommendationFactor` as a real policy, so `UpdateFromPolicy` runs for it and the factor reaches `AggregateContainerState` instead of the policy being skipped as a no-op.
- `deploy/vpa-v1-crd-gen.yaml` gets the new field and the updated `uncappedTarget` description to stay consistent with the Go types. The module installs its own CRD from `crds/`, which is maintained separately.
