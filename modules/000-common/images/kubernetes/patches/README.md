## Patches

Warning! Some required patches that extend kubernetes functionally located in `ee/modules/000-common/images/kubernetes/patches/`
directory.

### local-init-configuration.patch

We want to include in join data the following:

```yaml
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: { { .nodeIP | quote } }
  bindPort: 6443
```

> Consider finding a way to do it without patching the `kubeadm` or make a PR to the upstream.

### pdb-daemonset.patch

Supports DaemonSets in disruption controller by adding /scale subresource to daemonsets API. It allows to control the eviction rate of DaemonSet pods.

> Upstream PR https://github.com/kubernetes/kubernetes/pull/98307.

### fix-mount-hostaliases.patch

Fixes a bug where pods with hostNetwork ignored host aliases (k8s < 1.32):

> https://github.com/kubernetes/kubernetes/pull/126460

### resource-quota-ignore-mechanism.patch

Add resource quota ignore mechanism for k8s pvc and pod based on labels

### kubelet-graceful-shutdown-cleanup-memory-manager-state

This patch ensures that the Memory Manager state file is removed during a graceful node shutdown.

The Memory Manager stores the node memory state in a file. After a reboot, the amount of used memory may slightly differ from the previous state, which can make the stored state invalid and prevent the kubelet from starting. Removing the state file before shutdown ensures that the Memory Manager starts with a clean state after the reboot.
See issue: https://github.com/kubernetes/kubernetes/issues/131253

### kubelet-disable-k-panic-check

Kubelet strictly checks that the `kernel.panic` parameter equals 10, now, regardless of kubelet settings, only a warning is used. The `kernel.panic` parameter itself is strictly controlled by the DKP platform

### namespace-list-acl-filtering.patch

Two related mechanisms, both opt-in and both leaving default `list/get/watch`
behavior for any client that doesn't ask otherwise completely unchanged:

**Namespaces (unconditional).** Users without cluster-wide `list/get/watch
namespaces` receive an ACL-filtered response for `GET /api/v1/namespaces`,
`GET /api/v1/namespaces/{name}` and `WATCH /api/v1/namespaces` instead of a
flat 403. The kube-apiserver authorization filter bypasses the initial 403
for these three verbs and delegates filtering to the Namespace storage, which
queries the aggregated extension API `authorization.deckhouse.io/v1alpha1`
resource `accessiblenamespaces` served by `permission-browser-apiserver`
(APIService `v1alpha1.authorization.deckhouse.io`) and returns only
accessible namespaces. `watch` synthesizes `ADDED`/`DELETED` events when the
user's accessible-namespace set changes mid-watch (polling
`accessiblenamespaces`, ~1s cadence) -- the canonical OpenShift
`userProjectWatcher` pattern.

**Generic `-A --scope=<kind>` (opt-in, every built-in namespaced resource).**
The same mechanism generalized to arbitrary namespaced resources, gated by two
request headers (`X-Deckhouse-Scope`, `X-Deckhouse-Project`) rather than being
unconditional: absent header, absent bypass, byte-for-byte vanilla behavior.
`scope=accessible` reproduces the namespaces mechanism's RBAC-floor
semantics for any resource; `scope=system|user|project:<name>` additionally
classify by the `projects.deckhouse.io/project` namespace label
(multitenancy-manager's Project CRD), resolved via a direct loopback
`Namespace LIST` rather than a new permission-browser endpoint.

Coverage is wired centrally in `pkg/controlplane/apiserver`'s `InstallAPIs`,
which walks every built-in API group's storage map and attaches the filter to
each namespaced, non-subresource resource backed by a `*genericregistry.Store`
(via the promoted `DeckhouseScopeStore` method). No per-resource storage.go
edits. `namespaces` is excluded structurally (it wraps rather than embeds its
Store and keeps its own unconditional filter above).

Namespaced **CustomResourceDefinitions** are covered too, wired analogously in
apiextensions-apiserver's `crdHandler.getOrCreateServingInfoFor` as each CRD
version's storage is built (`customresource.REST` also embeds
`*genericregistry.Store` and does not override List/Get/Watch, so the same
`store.ScopeFilter` hook applies); registration is dropped on CRD teardown.
Because that wiring edits the same two apiextensions files that
`010-x-kubernetes-sensitive-data` also edits, and this patch (`005`) applies
before `010`, patch `010` is shipped **regenerated** so its
`customresource_handler.go` hunks apply cleanly on top of the CRD wiring — a
context-only re-roll, no semantic change to `010` (see its README entry).
Cluster-scoped CRDs are skipped (no namespace to classify/floor).

The filter's `RBACFloor`/`Classify` loopback calls are served through a shared
TTL cache (default 1s, `SCOPEFILTER_RESOLVE_CACHE_TTL` /
`SCOPEFILTER_WATCH_POLL_INTERVAL` to tune) with singleflight de-duplication, so
many concurrent `--scope` watches collapse to ~one `permission-browser` resolve
per key per TTL instead of one per watch per poll tick.

See `k8s.io/apiserver/pkg/registry/generic/scopefilter` (staging) for the
shared implementation both mechanisms build on.

If `permission-browser-apiserver` is not present/unavailable (APIService is
not `Available=True` or a request fails), both mechanisms fall back to
vanilla Kubernetes (403 for users without permissions).

### kubelet-inappropriate-manifest-name.patch

This patch ensures that files like `kube-apiserver.backup`, `kube-apiserver.yaml.bak`, or any other non-YAML files are not processed as static pod manifests,
this prevents kubelet from accidentally processing backup files or other non-manifest files in the `/etc/kubernetes/manifests directory`.
See issues:
- https://github.com/kubernetes/kubernetes/issues/55596
- https://github.com/kubernetes/kubernetes/issues/129364 -> https://github.com/kubernetes/kubernetes/pull/105695

### set-usage-GOPROXY.patch

Removes GOPROXY=off from the build so that our value is used when building the image.

### 010-x-kubernetes-sensitive-data.patch
x-kubernetes-sensitive-data marks a field as containing sensitive data (e.g. passwords, API keys).
When set the API server will: encrypt the value at rest in etcd (using the same transformer as Secrets) hide the field from get/list/watch responses unless the caller has RBAC permissions to access the sensitive subresource and unconditionally mask the value in audit logs.
See our KEP:
- https://github.com/kubernetes/enhancements/pull/5937
- https://github.com/kubernetes/enhancements/issues/5933

> Note (Deckhouse): the `apiserver.go` / `customresource_handler.go` hunks of
> this patch were regenerated so they apply cleanly on top of the CRD
> scope-filter wiring added by `namespace-list-acl-filtering.patch` (which
> applies earlier in the chain). This is a context-only re-roll — the
> sensitive-data feature itself is unchanged. If you update this patch,
> regenerate it against a tree that already has the earlier patch applied.

### 011-fix-stale-token-metrics.patch

Patch enhances the observability of stale ServiceAccount tokens by adding namespace and name labels to the `serviceaccount_stale_tokens_total` metric.

Why it is needed:
Previously, the `serviceaccount_stale_tokens_total` metric was a simple counter without any labels. While it indicated that some clients were using outdated tokens (past their warnafter threshold), it provided no information about which ServiceAccounts were responsible

### fix-scheduler-node-graceful-shutdown.patch

Patch prevents the scheduler from placing new pods on nodes that are in graceful shutdown.

Why it is needed:
During graceful node shutdown, kubelet reports the shutdown state through the `NodeReady` condition. The scheduler must treat such nodes as unschedulable and react to `NodeReady` condition updates so pods can be queued again when the shutdown state is cleared.

> Upstream PR https://github.com/kubernetes/kubernetes/pull/139249

### short-del-timeout-for-mirror.patch
Added a 5s timeout context wrapping the Delete call in DeleteMirrorPod, that kubelet can proceed immediately after the API call times out or succeeds, restoring normal static pod re-creation latency.

See issues:
- https://github.com/kubernetes/kubernetes/issues/139502
