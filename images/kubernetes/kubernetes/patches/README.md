## Patches

Warning! Some required patches that extend kubernetes functionally located in `ee/images/kubernetes/kubernetes/patches/`
directory.

### 000-gomod.patch

Bumps Go dependencies in the root `go.mod`/`go.sum` (and the `go` directive in `go.work` where needed) to fix known CVEs in the Kubernetes binaries, including `kube-proxy`. The build runs `GOWORK=off go mod vendor` after the patches are applied, so only the root module files are changed; staging modules pick up the new versions through MVS.

Current bumps on all versions: `golang.org/x/net` v0.62.0 (with `golang.org/x/crypto` v0.58.0 and `golang.org/x/text` v0.43.0), `google.golang.org/grpc` v1.83.2, `go.opentelemetry.io/otel`, `otel/sdk`, `otel/exporters/otlp/otlptrace` and `otlptracegrpc` v1.45.0 (with `go.opentelemetry.io/proto/otlp` v1.11.0). On 1.33-1.35 this moves `otlptracegrpc` from v1.33.0/v1.34.0 to v1.45.0 and drops the no longer used `github.com/cenkalti/backoff/v4`.

The patch is carried on 1.33 through 1.37 and must apply first (`000`), before patches that touch `go.work.sum` or source code. Regenerate it with `go get` on the upstream tag with the whole patch set applied. Remove the bumps once the upstream patch release ships the same or newer versions.

### old_certs.patch

Logs a warning from the kube-apiserver x509 client-certificate authenticator
when a request presents a client certificate that expires in less than seven
days. The line carries the request's method, URI, user agent and remote
address plus the certificate's issuer, subject, `NotBefore` and `NotAfter` —
enough to identify the client that needs rotating.

Why it is needed: upstream only feeds the remaining lifetime into the
`apiserver_client_certificate_expiration_seconds` histogram, right next to
where this patch hooks in. That histogram shows that *some* client is close to
expiry but cannot say which one — it carries no identifying labels, and could
not without unbounded cardinality.

### pdb-daemonset.patch

Supports DaemonSets in disruption controller by adding /scale subresource to daemonsets API. It allows to control the eviction rate of DaemonSet pods.

> Upstream PR https://github.com/kubernetes/kubernetes/pull/98307.

### resource-quota-ignore-mechanism.patch

Lets Deckhouse components leave Pods and PVCs fully or partly out of ResourceQuota usage with labels:

| Label (`resource-quota-overrides.deckhouse.io/...`) | Pod | PVC |
| --- | --- | --- |
| `ignore: "true"` | Not counted at all, `pods` and `count/pods` included | Not counted at all |
| `discount-cpu`, `discount-memory`, `discount-storage` | The value is subtracted from `requests.*` and `limits.*`, down to zero | No effect |
| `cap-cpu`, `cap-memory`, `cap-storage` | `requests.*` and `limits.*` are counted up to the value | No effect |

- The `*-storage` labels apply to the `ephemeral-storage` of a Pod, not to PVCs.
- `cpu`, `memory` and `ephemeral-storage`, the names a quota may use for `requests.*`, change together with `requests.*`.
- `ignore` goes first, then `discount`, then `cap`.
- The quota controller recounts the quota as soon as a label is added, removed or changed on an existing Pod or PVC, without waiting for the full resync.
- The API server rejects invalid values: `ignore` is `true` or `false`, CPU labels take CPU quantities (`500m`, `1`), memory and storage labels take byte quantities (`1Gi`), negative values are invalid.
- Only Deckhouse components may set the labels. The `resource-quota-overrides.deckhouse.io` and `resource-quota-overrides-templates.deckhouse.io` ValidatingAdmissionPolicies of control-plane-manager check Pods, PVCs and the templates controllers create them from.

The image build does not run the tests of the patch: run `go test -mod=readonly ./pkg/apis/core/validation/ ./pkg/quota/v1/evaluator/core/ ./pkg/quota/v1/install/ -run ResourceQuota` on the patched tree.

> Design: ADR `architecture-decision-records/dkp/2025-12-10-resource-quota-ignore-mechanism.md` in the docs repository.

### kubelet-disable-k-panic-check

Kubelet strictly checks that the `kernel.panic` parameter equals 10, now, regardless of kubelet settings, only a warning is used. The `kernel.panic` parameter itself is strictly controlled by the DKP platform

### kubelet-machine-resources-self-restart

Kubelet reads CPU and memory only at start, so node capacity does not follow CPU/memory hotplug in a VM.
Every 5 minutes (cAdvisor machine info refresh rate) kubelet compares the cAdvisor machine info with its own
start snapshot and exits (code 75) when CPUs grew or memory grew by 100 MiB or more; systemd restarts it with
the new capacity. Any decrease vetoes the restart: readmission would fail Pods on the shrunk resource.
No restart loop: the baseline is the process's own snapshot, so a restart happens only on a new maximum,
at most once per check interval. Disable by creating `<root-dir>/machine-resources-restart-disabled`. Skipped restarts and toggling the file are logged once per state change.

> Upstream KEP-3953 (node resource hot plug) is implemented started with 1.38. 

### namespace-list-acl-filtering.patch

Two related mechanisms, both opt-in and both leaving default `list/get/watch`
behavior for any client that doesn't ask otherwise completely unchanged:

**Namespaces (unconditional).** Users without cluster-wide `list/get/watch
namespaces` receive an ACL-filtered response for `GET /api/v1/namespaces`,
`GET /api/v1/namespaces/{name}` and `WATCH /api/v1/namespaces` instead of a
flat 403. The kube-apiserver authorization filter bypasses the initial 403
for these three verbs and delegates filtering to the Namespace storage, which
asks `permission-browser-apiserver` (APIService
`v1alpha1.authorization.deckhouse.io`) for the access boundary of `list
namespaces` (see below) and returns only the namespaces in it. permission-browser
answers that read with the same set its `accessiblenamespaces` resource serves;
kube-apiserver itself no longer calls `accessiblenamespaces`. `watch`
synthesizes `ADDED`/`DELETED` events when the user's accessible-namespace set
changes mid-watch (re-asking, ~1s cadence) -- the canonical OpenShift
`userProjectWatcher` pattern. `deckhouse.io` `projects` are filtered the same
way (a project is visible when any of its namespaces is).

**Generic `-A --scope=<kind>` (opt-in, every built-in namespaced resource).**
The same mechanism generalized to arbitrary namespaced resources, gated by two
request headers (`X-Deckhouse-Scope`, `X-Deckhouse-Project`) rather than being
unconditional: absent header, absent bypass, byte-for-byte vanilla behavior.
When RBAC denied the read, the result is limited to the RBAC floor: the access
boundary of the request's exact group, resource, subresource and verb.
`scope=accessible` applies only that floor; `scope=projects|project:<name>`
additionally classify by the `projects.deckhouse.io/project` namespace label
(multitenancy-manager's Project CRD), and `scope=system` by a fixed name-based
allowlist (`default`, `d8-*`, `kube-*`) independent of that label -- both
resolved via a direct loopback `Namespace LIST` rather than a
permission-browser call.

**Access boundary.** One `SelfScopedAccessReview` through the loopback client,
impersonating the caller: `scope: accessible` (kube-apiserver intersects the
classification itself) and a single request with `includeNamespaces: true`. The
result maps, in order: `evaluationError` or `reason` is an error (403);
`allowed: false` grants nothing, whatever `namespaces`/`cluster` it carries;
`cluster: true` on a denied request is a disagreement of the two authorization
chains and is refused with 403 (a namespace `get`/`watch` is then not
restricted, the projects listing shows every project); otherwise `namespaces`.

Coverage is wired centrally in `pkg/controlplane/apiserver`'s `InstallAPIs`,
which walks every built-in API group's storage map and attaches the filter to
each namespaced, non-subresource resource backed by a `*genericregistry.Store`
(via the promoted `DeckhouseScopeStore` method). No per-resource storage.go
edits. `namespaces` is excluded structurally (it wraps rather than embeds its
Store and keeps its own unconditional filter above). `roles` and
`rolebindings` are served through `policybased.Storage`, which embeds the
`rest.StandardStorage` interface and so hides the method: the wiring unwraps
it (the exported `StandardStorage` field) and attaches the filter to the inner
Store. The wrapper only adds escalation checks to create and update, so reads
go through the inner Store unchanged.

Namespaced **CustomResourceDefinitions** are covered too, wired analogously in
apiextensions-apiserver's `crdHandler.getOrCreateServingInfoFor` as each CRD
version's storage is built (`customresource.REST` also embeds
`*genericregistry.Store` and does not override List/Get/Watch, so the same
`store.ScopeFilter` hook applies); registration is dropped on CRD teardown.
Because that wiring edits the same two apiextensions files that
`010-x-kubernetes-sensitive-data` also edits, and the acl-filtering patch
applies before `010` in the chain, patch `010` is shipped **regenerated** so
its `customresource_handler.go` hunks apply cleanly on top of the CRD wiring —
a context-only re-roll, no semantic change to `010` (see its README entry).
Cluster-scoped CRDs are skipped (no namespace to classify/floor).

This whole mechanism (namespaces + generic `--scope` + CRD coverage) is carried
on every k8s version DKP builds — 1.33 through 1.37. The acl-filtering patch
keeps its per-version number (`005` on 1.33/1.34, `007` on 1.35/1.36/1.37)
since its position in each version's chain differs; the `010` re-roll applies
per version too.

The filter's loopback calls (the review and the `Namespace LIST`) are served
through a shared TTL cache (default 1s, `SCOPEFILTER_RESOLVE_CACHE_TTL` /
`SCOPEFILTER_WATCH_POLL_INTERVAL` to tune) with singleflight de-duplication, so
many concurrent `--scope` watches collapse to ~one review per key per TTL
instead of one per watch per poll tick.

See `k8s.io/apiserver/pkg/registry/generic/scopefilter` (staging) for the
shared implementation both mechanisms build on.

Without the access boundary a denied read serves no objects (fail closed): a
review that times out or gets 429 or a 5xx (also when the APIService is not
`Available=True`) answers 503 with `Retry-After`, any other failure 403. Only
when the filter was not built (no loopback config, resolvers failed) do the
reads behave as in vanilla Kubernetes.

### scope-aggregated-api.patch

Extends the generic `-A --scope=<kind>` mechanism of `namespace-list-acl-filtering.patch` to
resources served by aggregated API servers, which have no storage in kube-apiserver. An `APIService`
opts resources in with the `scope.deckhouse.io/resources` annotation (comma-separated plural names
within `spec.group`; refused for `authorization.deckhouse.io` and for local APIServices). For a scoped
`list/get/watch` of such a resource the authorization filter lets an RBAC-denied read through, and
kube-aggregator computes the allowed set of namespaces with the same decision as the storage filter
(classification ∩ RBAC floor from `SelfScopedAccessReview`) and sends it to the extension server in
the trusted headers `X-Deckhouse-Scope-Mode` (`filtered`/`narrowed`) and
`X-Deckhouse-Scope-Namespaces`; the extension filters. kube-aggregator strips client copies of these
headers from every proxied request, answers a denied `get` outside the set with 404 itself, refuses a
set that does not fit the 1 MiB header limit with 400, and fails closed (403/503) when the set cannot
be computed. A scoped watch is ended when its set changes or the opt-in is removed; over HTTP it first gets a
`410 Gone` error event, so the client lists again with the new set.
Without the annotation, behavior is unchanged.

The patch depends on `namespace-list-acl-filtering.patch` and applies after it. It is carried on
1.33 (`023`), 1.34 (`024`), 1.35 (`023`), 1.36 (`022`) and 1.37 (`021`). The contract an extension server must implement (trust the headers only on front-proxy
authenticated requests, and so on), watch semantics, metrics and tests are described in
[SCOPE_AGGREGATED_API.md](../../../../modules/140-user-authz/docs/internal/SCOPE_AGGREGATED_API.md).

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


### fix-container-memory-usage-in-resize-validation.patch (1.34+)

Fixes a kubelet panic in `validateMemoryResizeAction`
(`pkg/kubelet/kuberuntime/kuberuntime_manager.go`): the per-container check
dereferences `podUsageStats.Memory.UsageBytes` instead of
`cStats.Memory.UsageBytes`. That pointer is nil-checked only in the
`podLimitDecreasing` branch, so an in-place resize decreasing **only a
container** memory limit panics kubelet when the runtime reports no pod-level
memory stats.

Only the source change is carried, the upstream test case is dropped.
`validateMemoryResizeAction` does not exist in 1.33, so the patch is
carried on 1.34 (`014`), 1.35 (`014`), 1.36 (`013`) and 1.37 (`013`) only.

> Upstream PR https://github.com/kubernetes/kubernetes/pull/141100

### order-sandbox-by-attempt-or-create-time.patch (1.33-1.34)

Makes `podSandboxByCreated.Less` order sandboxes by `Metadata.Attempt` instead of
`CreatedAt`, falling back to `CreatedAt` when either sandbox has no metadata
(`pkg/kubelet/kuberuntime/helpers.go`).

A system clock rollback (common at boot, when NTP starts around the static pods)
can make an older sandbox look newer than the current one. Kubelet then picks the
stale sandbox as the latest one, and the runtime refuses to create a new sandbox
because the pod name is still reserved by the previous attempt. The attempt
counter is monotonic and immune to clock jumps, so it is the correct ordering key.

Only the source change is carried, the upstream test case is dropped. k8s 1.35+
already ship this code, so the patch is carried on 1.33 (`014`) and 1.34 (`015`)
only.

> Upstream PR https://github.com/kubernetes/kubernetes/pull/130551

### kubelet-checkpoint-state-self-heal.patch (1.33+)

When the CPU or memory manager checkpoint is unusable, the kubelet resets that manager's state and starts instead of exiting. Running containers are not stopped.

The CPU manager moves running containers onto their new CPUs by itself. The memory manager cannot change a running container's NUMA zone, so such containers are reported until they are recreated: gauge `kubelet_checkpoint_state_unpinned_containers`, event `NUMACheckpointReset` and alert `D8KubeletCheckpointStateUnpinnedContainers`.

See issue: https://github.com/kubernetes/kubernetes/issues/131253

### json-patch-require-value.patch

Rejects JSON Patch operations without the `value` member where `gopkg.in/evanphx/json-patch.v4` dereferences a nil value and kube-apiserver panics while applying the patch:

- `add` and `replace` that target the whole document (`"path": ""`), in v4.12.0 and v4.13.0;
- `test` with any path. v4.12.0 panics when the tested field is missing or `null`; v4.13.0 does not panic but lets such a test pass, which makes no sense either.

The check runs before `Patch.Apply` in every place kube-apiserver applies a JSON Patch:

- `PATCH` requests with `application/json-patch+json` (`pkg/endpoints/handlers/patch.go`), answered with `400 Bad Request`;
- patches returned by mutating admission webhooks (`pkg/admission/plugin/webhook/mutating/dispatcher.go`);
- patches produced by MutatingAdmissionPolicy (`pkg/admission/plugin/policy/mutating/patch/json_patch.go`).

The shared check lives in a new package, `k8s.io/apiserver/pkg/util/jsonpatch`. It checks whether the `value` key is present, so an explicit `"value": null` is still accepted.

`add` and `replace` operations with a non-empty path and no `value` are deliberately left as they are: the library treats the missing value as `null` there. RFC 6902 requires `value`, but rejecting such patches would break existing clients.

The patch is carried on 1.33 (`016`), 1.34 (`017`), 1.35 (`016`) and 1.36 (`015`).

### limit-field-manager-validation-errors.patch

Bounds the errors `ValidateFieldManager` (`k8s.io/apimachinery/pkg/apis/meta/v1/validation`) returns for an invalid field manager:

- a value longer than 128 bytes gets a single `FieldValueTooLong` error, and its characters are not checked;
- a shorter value gets an error for the first non-printable character only.

Before the patch, every non-printable character produced its own `FieldValueInvalid` error, and each of these errors carried the whole value. A long `fieldManager` query parameter of a create, update or patch request, or a long `managedFields[].manager` in an object, made kube-apiserver build an error list that grows quadratically with the value length and join it into the `422` message. Such a request took seconds of CPU and a lot of memory, and the response was huge. Now the `422` response contains a single cause. Valid values are not affected.

### kubelet-static-pod-uid-from-manifest.patch

The kubelet derives the UID of a static pod from the manifest file content instead of the internal `api.Pod` object. Without the patch, every in-place kubelet minor upgrade or downgrade kills all static pods of the node (etcd, kube-apiserver, kube-controller-manager, kube-scheduler, kubernetes-api-proxy) without a graceful shutdown and starts them again.

Upstream hashes a dump of the internal `api.Pod` after defaulting (`applyDefaults` in `pkg/kubelet/config/common.go`). Every minor release adds fields to the internal types, so an unchanged manifest gets a new UID. The new kubelet treats the running pod with the old UID as an orphan and kills it with a 2-second grace period. The pod cgroup is destroyed in the same cleanup pass, which sends SIGKILL: etcd and kube-apiserver usually die with exit code 137; components that exit quickly on SIGTERM sometimes manage to, but without their graceful shutdown procedure.

With the patch, when the manifest has no `metadata.uid`, the UID is an FNV-128a hash of `utilyaml.ToJSON(<file>)` (exactly the bytes the kubelet decodes the pod from), the node name and the file path:

- the UID does not depend on the kubelet version, so the new kubelet keeps the running static pods;
- any change of the decoded pod changes the UID, so the pod is replaced and the mirror pod is recreated as before;
- for a YAML manifest, comments, indentation and key order do not change the UID; a JSON manifest is hashed as is, so reformatting it restarts the pod;
- an explicit `metadata.uid` is used as is;
- the static pod URL source is not changed.

The first start of the patched kubelet, and a return to a kubelet without the patch, changes the UIDs once and kills the static pods, as a minor upgrade does today. The kubelet package changes with the patch, so this happens on the Deckhouse release update that delivers the patch, on all nodes, even without a Kubernetes version change.

`TestStaticPodContentUIDGolden` and `TestStaticPodContentUIDGoldenFiles` pin the UIDs of fixed manifests (including Deckhouse-rendered `etcd.yaml` and `kube-apiserver.yaml` in `testdata/static-pod-uid`), and `TestStaticPodContentUIDTrickyJSON` pins the YAML to JSON conversion. They must pass unchanged in every minor version: if they fail, all static pods will be restarted on the upgrade to that version. The image build does not run these tests: run `go test ./pkg/kubelet/config/ -run 'StaticPod|ExtractFromFile'` on the patched tree when the patch is carried to a new minor version.

See issues:
- https://github.com/kubernetes/kubernetes/issues/67537
- https://github.com/kubernetes/kubernetes/issues/116080

### kubelet-cpumanager-smtalign-preallocated.patch (1.33-1.36)

Backport of the CPU manager fix from [kubernetes/kubernetes#129079](https://github.com/kubernetes/kubernetes/pull/129079) (commit `bbf1e0550ed`, released in 1.37).

With `cpuManagerPolicyOptions: full-pcpus-only: "true"` the static policy checks SMT alignment before it looks up the checkpoint. After a kubelet restart the CPUs of running containers are already taken in the restored state, so `availablePhysicalCPUs` does not include them. A running container that holds more CPUs than are left free is rejected with `SMTAlignmentError` and killed, with or without SMT. The patch moves the "container already present in state" check before the SMT check, the same as upstream.

1.33-1.35 still have the SMT check inline in `Allocate` (upstream later moved it to `enforceSMTAlignment`), so only the order of the two blocks changes; 1.36 takes the upstream change as is. The upstream unit test is carried. 1.37+ already ships the fix. `NodeGroup` `spec.kubelet.topologyManager` with policy `Restricted` or `SingleNumaNode` enables `full-pcpus-only`, so the patch is needed there.

### license-warning.patch

While a Deckhouse cluster does not have enough licenses, kube-apiserver adds a warning to the resource responses of every user who is not a `system:` one. The request is served as usual, kubectl prints the warning to stderr:

```console
$ kubectl get nodes
Warning: Deckhouse: the cluster does not have enough licenses. Please add licenses.
```

- A post-start hook reads `EffectiveLicense` `deckhouse-platform` (`deckhouse.io/v1alpha1`) through the loopback client every minute (the first time a minute after start, when the CRD informer is synced) and keeps its `status.apiWarning`. The licensing controller of deckhouse-controller computes this field, the patch has no rules or clock of its own. No CRD or no object (404) means no warning, so editions and clusters without licensing are not affected. Any other error keeps the last known state and is logged at most once per 10 minutes.
- A filter wraps the API handler of the aggregator (`BuildHandlerChainFunc` in `CreateAggregatorConfig`). It runs after authentication, impersonation and authorization for every API, including CRDs and aggregated APIs. It adds the warning to resource requests (`RequestInfo.IsResourceRequest`) when the user name does not start with `system:`, so service accounts, nodes and control plane components do not get it and every human client does: kubectl under any name, d8, Helm, werf, k9s, Lens, Headlamp, oc, curl. The only exception is dhctl (User-Agent starting with `dhctl/`): it works under the admin kubeconfig, makes hundreds of requests per run and would only write the warning to its log on every one, client-go logs every warning by default.

Discovery (`/api`, `/apis`, group versions), `/openapi/*`, `/version` and other non-resource paths never get the warning. kubectl and Helm keep discovery and OpenAPI responses in `~/.kube/cache/http` with all headers and revalidate them by ETag: on a 304 the stored `Warning` stays, and OpenAPI v3 `?hash=` responses are served from the cache without a request for a year. A warning there would outlive the license problem. Resource responses carry `Cache-Control: no-cache, private` and no ETag, and client-go does not cache them, so their warning is always current. Commands that make no resource requests (`kubectl version`, `api-resources`, `api-versions`, `explain`) print no warning.

While the warning is active, `kubectl --warnings-as-errors` fails on every command that requests resources. A watch gets the warning once, when it starts.

The code is in a new package, `pkg/controlplane/apiserver/licensewarning`, `server.go` and `aggregator.go` only wire it.

### kubelet-host-network-hosts-aliases-first.patch

For `hostNetwork` Pods kubelet builds `/etc/hosts` from a copy of the node's `/etc/hosts` and appends `spec.hostAliases` after it. Resolvers take the first matching line, so an entry added to the node file by hand (for example, a stale address of Dex) shadows the Pod's `hostAliases` and breaks authentication. The patch writes the `hostAliases` block right after the kubelet header, before the node entries. Pods without `hostAliases` and Pods without `hostNetwork` get the same file as before.

### scheduler-simulate-endpoint.patch

Adds a dry-run scheduling endpoint to kube-scheduler: `POST /simulate` answers "which node would this Pod get right now, and why". Without it nobody outside the scheduler can tell where a Pod will land or why it landed there: the descheduler evicts blindly, and the reasons for a placement are visible only in the scheduler's debug logs.

The scheduler evaluates the Pod as in a real scheduling cycle, on its live cache, with the same plugins and extenders, but places nothing and changes nothing. The answer lists every node: whether it fits, which filter or extender rejected it and why, and the score each plugin and extender gave it. Scores are named `plugin/<Name>` and `extender/<name>`, as in the decision log of `scheduler-decision-observability.patch`; an extender that rejected a node is `extender/<name>` too, a filter plugin keeps its plain name. `<name>` is the name of the extender in its metrics (`scheduler-extender-metrics.patch`), or its URL prefix when it has no KubeSchedulerWebhookConfiguration. The caller can exclude nodes (the source node of a live migration) and treat some Pods as already deleted (the evicted Pod, or all Pods of a node that is about to be removed). Query parameters add details: `weights=true` shows the weight behind every score, `filters=all` lists every filter a node fails instead of the first one.

Extenders get the filter and prioritize requests of a simulation with `"dryRun": true` (`scheduler-extender-dry-run.patch`). While a simulation runs, real scheduling waits for it, extender calls included; therefore a replica runs one simulation at a time and answers 429 to a concurrent request. `filters=all` makes a simulation longer and is meant for debugging.

The endpoint listens on loopback only. `control-plane-proxy` publishes it as `/scheduler/simulate`; access is granted by the ClusterRole `d8:control-plane-manager:scheduler-simulate`.

The patch is carried on 1.33 (`024`), 1.34 (`025`), 1.35 (`024`), 1.36 (`023`) and 1.37 (`022`).

### scheduler-extender-metrics.patch

Upstream kube-scheduler has no metrics for HTTP extenders: a slow or failing extender shows up only as a longer scheduling cycle, and a dropped prioritize call is silent. The patch adds:

- `scheduler_extender_request_duration_seconds{extender, verb, result}`: every request to an extender with its result: `success`, `extender_error` (the extender answered with an error in the body), `timeout`, `error` (connection or local failure), `status` (HTTP code other than 200), `decode`;
- `scheduler_extender_request_phase_duration_seconds{extender, verb, phase}`: where a successful request spends its time (encoding, waiting for a connection, writing, waiting for the answer, reading);
- `scheduler_extender_info{extender, url, filter_verb, prioritize_verb, preempt_verb, bind_verb, weight, ignorable}`: the configured extenders with their URL prefixes, published by every replica;
- `scheduler_extender_filter_nodes_total{extender, outcome}`: nodes a filter answer passed, failed or failed as unresolvable.

Requests made by the simulate endpoint are not recorded.

`extender` is the name of the extender: the name of its KubeSchedulerWebhookConfiguration, with `/<n>` (the position of the webhook in `webhooks`, from 1) when the object has several webhooks. Deckhouse passes the names in `DECKHOUSE_EXTENDER_NAMES` (comma-separated `<urlPrefix>=<name>`); an extender without a name, for example one configured by hand, is labelled by its URL prefix. The number of series stays the same, but the label values change once, on the upgrade to a Deckhouse that renders the names: graphs and alerts by `extender` break at that point.

The patch is carried on 1.33 (`025`), 1.34 (`026`), 1.35 (`025`), 1.36 (`024`) and 1.37 (`023`).

### scheduler-extender-circuit-breaker.patch

A failing or hung extender with `failurePolicy: Ignore` costs its whole timeout in every scheduling cycle, and its answer is dropped anyway. After 10 failed filter or prioritize requests in a row kube-scheduler stops sending filter and prioritize requests to such an extender for 30 seconds, then sends one probe request: success resumes the calls, failure pauses them again. Bind and preempt requests are never skipped, an answer with an error from the extender is not a failure, and extenders with `failurePolicy: Fail` are never skipped. Skipped requests are counted with `result="skipped"`. `scheduler_extender_circuit_open{extender}` shows whether an extender is skipped now, `scheduler_extender_circuit_open_total{extender}` how often it happened; `extender` is the extender name, as in the metrics of `scheduler-extender-metrics`. The simulate endpoint skips a paused extender too, but never changes the breaker.

The patch is carried on 1.33 (`026`), 1.34 (`027`), 1.35 (`026`), 1.36 (`025`) and 1.37 (`024`).

### scheduler-extender-placements.patch

An extender that scores nodes by their load learns about a placed Pod only when its binding reaches the extender, about a second later under load, so a burst of Pods is scored as if none of them were placed. For an extender with `sendPlacements: true` in its KubeSchedulerWebhookConfiguration, kube-scheduler adds the `assumed` field to the next `prioritize` request with information about tentative Pod reservations on nodes and their cancellations.

The `assumed` field contains a list of objects with the following fields:

- `uid` — UID of the Pod;
- `namespace` — namespace of the Pod;
- `name` — name of the Pod;
- `node` — name of the node;
- `forget` — optional field. A value of `true` indicates that the reservation was canceled.

Placements go out only with a prioritize request: kube-scheduler skips prioritize when a single node fits, so placements may wait for a later Pod. Placements older than 30 seconds are not sent; `scheduler_extender_placements_dropped_total{extender, reason}` counts them; `extender` is the extender name, as in the metrics of `scheduler-extender-metrics`. Deckhouse lists the extenders with `sendPlacements: true` in `DECKHOUSE_EXTENDER_PLACEMENTS` by their URL prefixes: the names are used only in metric labels. Other extenders get the usual request. An extender must accept repeated placements (entries from a failed request may be sent again), a `forget` for a Pod it does not know, and must expire a placement after 30 seconds if no `forget` arrives.

The patch is carried on 1.33 (`027`), 1.34 (`028`), 1.35 (`027`), 1.36 (`026`) and 1.37 (`025`).

### scheduler-extender-dry-run.patch

When the simulate endpoint evaluates a Pod, kube-scheduler calls the extenders as in a real cycle. Before this patch an extender could tell such a call only by the annotation `scheduler.deckhouse.io/simulation` on the Pod copy, and any user can put that annotation on a real Pod. Now the filter and prioritize requests of a simulation carry `"dryRun": true`, and the scheduler no longer adds the simulation annotation to the Pod copy. On `dryRun`, an extender computes its answer and does not cause side effects, such as reservations. Requests of real scheduling are unchanged and have no `dryRun` field. An extender that decodes requests leniently ignores the field; an extender that rejects unknown JSON fields (Go `DisallowUnknownFields`) fails the requests of a simulation, while its real requests are not affected. A simulation never sends preempt or bind requests.

kube-scheduler tells its own simulation by the Pod copy object. A real Pod with the annotation is processed during scheduling, recorded in extender metrics, counted by the circuit breaker and sent in placements like any other Pod.

The patch is carried on 1.33 (`028`), 1.34 (`029`), 1.35 (`028`), 1.36 (`027`) and 1.37 (`026`).

### scheduler-decision-observability.patch

Upstream kube-scheduler does not say why a Pod landed on its node: the scores are visible only in debug logs at level 10, and the simulate endpoint shows what would happen now, not what happened. The patch explains the real decisions and measures the simulate endpoint.

After a scheduling cycle selects the node of a Pod, kube-scheduler compares it with the best other feasible node by total score (the runner-up). The score plugin or prioritize extender with the largest positive difference of weighted scores between the two decided. Simulations are not counted. Pods placed by the pod-group cycle (1.36+) produce no decision at all, neither metrics nor event text nor log entry: the cycle runs `schedulePod` for every candidate placement and binds the kept one without another run.

"Feasible nodes" are the nodes the filters found. With `percentageOfNodesToScore` (adaptive by default) the filters stop after enough feasible nodes, for example about 500 of 5000, so `scheduler_decision_feasible_nodes`, the runner-up and `tie` refer to that sample, not to all nodes of the cluster.

Metrics (`profile` is the scheduler profile):

| Metric | Labels | Series |
|---|---|---|
| `scheduler_decision_deciding_plugin_total` | `profile`, `plugin` | profiles × (score plugins + prioritize extenders + 4) |
| `scheduler_decision_score_margin` (histogram: total score of the selected node minus that of the runner-up; buckets 0, 1, 2, 5 ... 5000) | `profile` | profiles × 16 |
| `scheduler_decision_feasible_nodes` (histogram: feasible nodes of the decision; buckets 1, 2, 5 ... 5000) | `profile` | profiles × 15 |
| `scheduler_simulate_requests_total` | `code`: 200, 400, 404, 405, 429, 500, 503, 504 | 8 |
| `scheduler_simulate_duration_seconds` (histogram: time a simulation holds the scheduling lock; real cycles wait meanwhile; 1ms .. 33s) | `filters`: `first`, `all` | 2 × 19 |
| `scheduler_simulate_lock_wait_seconds` (histogram: time a simulation waits for the lock; 1ms .. 33s) | none | 19 |

`plugin` says what decided, with a prefix:

- `plugin/<name>`: a score plugin, for example `plugin/NodeResourcesFit`;
- `extender/<name>`: a prioritize extender by its name, the `extender` label of the extender metrics (`scheduler-extender-metrics`): the KubeSchedulerWebhookConfiguration name, or the URL prefix for an extender without a name;
- `scheduler/single-node`: only one node passed the filters, there was no choice;
- `scheduler/tie`: scoring ran, but the best nodes have equal totals; the winner was picked at random;
- `scheduler/nominated-node`: the Pod took the node nominated for it during preemption; it was tried first and fit, other nodes were not evaluated;
- `scheduler/node-hint`: since 1.35 the scheduler took the node from the OpportunisticBatching hint, left by an earlier Pod of the same shape, without scoring; other nodes were not evaluated.

The number of series does not depend on the prefixes; the values changed once (from `single_feasible_node`, `tie`, `nominated_node`, `node_hint`, plugin names and extender URL prefixes), so graphs and alerts by `plugin` break at the upgrade to this version of the patch.

A decision is counted when the node is selected, also when the cycle fails later (Reserve, Permit, binding). `scheduler_decision_feasible_nodes` is not observed for `scheduler/nominated-node` and `scheduler/node-hint`, `scheduler_decision_score_margin` only when nodes were scored.

Two outputs are off by default and read once at start from the environment of kube-scheduler (Deckhouse templates do not set them yet):

- `DECKHOUSE_SCHEDULER_DECISION_EVENTS=true` appends the top three nodes with their total scores and the deciding plugin to the message of the `Scheduled` event: `Successfully assigned ns/pod to node-a; top: node-a=512 node-b=498 node-c=470; decided by plugin/NodeResourcesFit (+14)`, or with an extender `...; top: a=570 b=448; decided by extender/usage-extender (+80)`. The message stays within the 1024 bytes the events API accepts: nodes that do not fit are left out. Without the variable the message is the upstream one.
- `DECKHOUSE_SCHEDULER_DECISION_LOG_SIZE=<n>` keeps the last `n` decisions in memory (at most 10000). An entry keeps at most 32 scores per node: about 2.6 KB in the worst case, 26 MB for 10000 entries; with 17 score plugins and extenders about 1.5 KB, 15 MB for 10000. `GET /decisions?namespace=&name=&limit=` on the loopback listener of the simulate endpoint returns them newest first: time, Pod, profile, node, number of feasible nodes, deciding plugin, its lead and the margin, the duration of the scheduling attempt up to the decision, and the top three nodes with the weighted score of every plugin and extender; `decidedBy` and the score names use the same prefixed names as the metric, and so does the answer of `/simulate`. `limit` is 100 by default and at most 1000 (and `n`), a larger value is cut; the answer is written entry by entry, never built in one buffer. Without the variable the endpoint answers 404. The log needs `DECKHOUSE_SCHEDULER_SIMULATE_ADDRESS`, as the simulate endpoint does.

`control-plane-proxy` publishes the log as `/scheduler/decisions`; access is granted by the ClusterRole `d8:control-plane-manager:scheduler-decisions`.

On the scheduling path a decision costs one pass over the scores of the feasible nodes and three metric updates, without allocations: 3-3.6 µs at 5000 nodes, about 0.1% of scoring the same nodes with trivial plugins (`BenchmarkDeckhouseDecision`, `BenchmarkDeckhousePrioritizeNodes`).

The patch is carried on 1.33 (`029`), 1.34 (`030`), 1.35 (`029`), 1.36 (`028`) and 1.37 (`027`).
