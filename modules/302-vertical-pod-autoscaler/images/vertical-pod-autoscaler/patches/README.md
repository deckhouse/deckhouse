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

## 005-admission-controller-disable-http2.patch

Serves the admission-controller webhook over HTTP/1.1 only (`http.Server.Protocols` with HTTP/1 alone).

- The webhook port is reachable from any pod through the `vpa-webhook` Service, and the server does not require client certificates. With HTTP/2 enabled, any pod could exploit HTTP/2 frame-level DoS issues such as CVE-2026-78669.
- kube-apiserver negotiates HTTP/1.1 via ALPN and keeps calling the webhook as before.

## 006-admission-controller-dedicated-webhook-mux.patch

Serves the admission-controller webhook from a dedicated `http.ServeMux` instead of `http.DefaultServeMux`.

- `pkg/utils/server` imports `net/http/pprof`, whose `init()` registers `GET /debug/pprof/*` on `http.DefaultServeMux`. The webhook server used that mux, so any pod could reach pprof on port 8000 through the `vpa-webhook` Service without authentication.
- The pprof handlers parse request forms, so multipart parsing issues such as CVE-2026-94440 were reachable from any pod.
- Profiling is still available on the metrics port with `--profiling`.
