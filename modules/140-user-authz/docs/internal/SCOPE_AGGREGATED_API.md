# Scoped reads of aggregated APIs (`--scope` for APIService extensions)

Audience: teams that own an aggregated API server (an `APIService` with `spec.service`) and want
`d8 k get <resource> -A --scope=...` to work for its resources, and platform-security maintainers of
the kube-apiserver patch `scope-aggregated-api.patch`.

The user-facing side of `--scope` (scopes, headers `X-Deckhouse-Scope`/`X-Deckhouse-Project`,
answers) is described in
[Filtered listing](../../../../docs/documentation/pages/admin/configuration/access/authorization/FILTERED-LISTING.md).
This document covers only what changes when the resource is served by an aggregated API server.

## The problem

For built-in resources and CRDs the scope filter lives in the kube-apiserver storage: a read that RBAC
denies is let through by the authorization filter and filtered by the storage decorator. An aggregated
API has no storage in kube-apiserver — kube-aggregator only proxies the request. Without this patch a
denied `list` of such a resource with `--scope` gets a plain `403`, and an allowed one is forwarded
unfiltered.

With the patch, kube-apiserver computes the allowed set of namespaces itself and sends it to the
extension server in trusted headers. The extension filters by that set.

## How it works

1. **Opt-in.** The `APIService` carries the annotation
   `scope.deckhouse.io/resources: <plural>[,<plural>...]` — plural resource names within
   `spec.group`. Entries are trimmed, deduplicated, and must be DNS labels; invalid entries (a `/`,
   an empty entry) are ignored with a warning in the kube-apiserver log. The opt-in is per
   `APIService`, so per version: a resource stays registered while any `APIService` of the group lists
   it, but each version's proxy only filters what its own `APIService` lists.
2. **Authorization bypass.** For a `list`, `get` or `watch` without a subresource that carries a valid
   `X-Deckhouse-Scope` header, the authorization filter checks whether the resource is stored in
   kube-apiserver with a scope filter; if not, whether some `APIService` opted it in. If RBAC denied
   the read and the resource is opted in, the request is let through to kube-aggregator, tagged as
   bypassed. A resource stored in kube-apiserver always wins over the opt-in.
3. **Allowed set.** kube-aggregator computes the set with the same function as the storage decorator:
   - the classification of the scope (`projects`, `project:<name>`, `system`) — not computed for
     `accessible`;
   - for a bypassed read only, the RBAC floor: the namespaces where the actual resource and verb are
     allowed, from one `SelfScopedAccessReview` of `permission-browser-apiserver`;
   - the set is the intersection of the two when both apply. A review result with `allowed: false`
     grants nothing, whatever namespaces it lists.

   `permission-browser-apiserver` reads the same annotation from its `APIService` index and answers
   such a review with the namespaces where RBAC allows the verb; a read of an aggregated resource that
   is not opted in stays `NotFilterable` (403 in kube-apiserver). The index follows `APIService`
   objects through an informer, so an opt-in takes effect within seconds; until then denied callers
   get 403, never an unfiltered read. A cluster-scoped resource listed in the annotation is answered
   with `NotFilterable` and logged once. After the annotation is removed, kube-apiserver stops
   bypassing at once, while the console may still see the namespaces until the index gets the event.
4. **Mode.** `filtered` — RBAC denied the read, the set is the boundary of what may be served at all.
   `narrowed` — RBAC allowed the read, the caller only narrows the view.
5. **Headers.** kube-aggregator sets on the proxied request:

   | Header | Value |
   |---|---|
   | `X-Deckhouse-Scope-Mode` | `filtered` or `narrowed` |
   | `X-Deckhouse-Scope-Namespaces` | Sorted namespace names joined by `,`. Empty value means the empty set |

   Both are always sent together. The identity headers (`X-Remote-User`, `X-Remote-Group`,
   `X-Remote-Extra-*`) and the front-proxy client certificate are added as for any proxied request.

### When headers are sent

| Caller | `accessible` | `projects`, `project:<name>`, `system` |
|---|---|---|
| RBAC allows the read | No headers, the request is forwarded as is | `narrowed` + classification |
| RBAC denies the read | `filtered` + RBAC floor | `filtered` + classification ∩ RBAC floor |

An allowed caller whose scope hits a resource this `APIService` version did not opt in gets the
request forwarded without headers — the scope is ignored, as for a built-in resource without the
filter. A request without `X-Deckhouse-Scope` never gets the headers.

### What kube-apiserver answers itself

| Situation | Answer |
|---|---|
| Namespaced URL, `get`, namespace outside the set, `filtered` mode | `404 NotFound`, the request is not forwarded |
| Namespaced URL, `list`/`watch`, namespace outside the set (either mode) | Forwarded with the empty set; the extension must return an empty result |
| Namespaced URL, `get`, namespace outside the set, `narrowed` mode | Forwarded with the empty set; the extension must answer `404` |
| Headers would exceed the receiver limit (see below) | `400 BadRequest`, asks for a narrower `--scope` |
| A namespace name in the set is not a DNS label | `500 InternalError`; an open watch ends |
| Scope `projects`/`project` while multitenancy-manager is disabled | `400 BadRequest` |
| `permission-browser-apiserver` times out, answers `429` or `5xx` | `503` with `Retry-After` |
| RBAC denies the read, but the permission service allows it cluster-wide | `403` (the two chains disagree) |
| Any other failure to compute the set | `403` |
| A bypassed read reaches an `APIService` version that does not filter the resource (annotation removed, another version opted in) | `403` "the aggregated API server does not filter this resource by scope" |
| A bypassed read without a computed RBAC floor | `403` |
| Malformed `X-Deckhouse-Scope`/`X-Deckhouse-Project` | `400`, before authorization (unchanged) |

None of these answers names the scope or the set. kube-aggregator never forwards a read RBAC denied
without the set to filter it by.

**Header size limit.** `k8s.io/apiserver` secure serving sets `MaxHeaderBytes` to 1 MiB, and its HTTP/2
server derives the header list limit from it. kube-aggregator estimates the size of the outgoing request
headers — request line, `Host`, every header including the `X-Remote-*` headers the transport adds,
32 bytes of overhead per field — and refuses the read with `400` when it exceeds 1 MiB minus a 16 KiB
margin. An extension server with a smaller header limit will reject large sets on its own.

### Cases that keep the vanilla behaviour

- No `APIService` opted the resource in: a denied scoped read gets the plain `403`.
- kube-aggregator started without scope resolvers (no loopback config): every opt-in is ignored and
  logged at `V(2)`; scoped reads behave as without the patch.
- A local `APIService` (no `spec.service`) cannot opt in, see [Threat model](#threat-model).

## Receiver contract

An extension server that lists a resource in `scope.deckhouse.io/resources` MUST:

1. **Trust the headers only on a request authenticated by its requestheader (front-proxy)
   authenticator:** a client certificate that chains to the requestheader client CA (published in
   `kube-system/extension-apiserver-authentication`, key `requestheader-client-ca-file`), a common name
   from `requestheader-allowed-names` when that list is not empty, and a non-empty username header
   (`X-Remote-User` by default; take the header names from the same ConfigMap). On any other request —
   direct client, bearer token, certificate of another CA — ignore the headers and authorize as usual.
   kube-apiserver strips client copies only from requests it proxies; a request that reaches the
   Service directly can carry anything.
2. **No `X-Deckhouse-Scope-Mode`:** no restriction, serve as usual.
3. **`X-Deckhouse-Scope-Mode` present:** serve only objects in a namespace listed in
   `X-Deckhouse-Scope-Namespaces`. An empty or missing `X-Deckhouse-Scope-Namespaces` is the empty
   set: serve nothing.
4. **Unknown mode value:** refuse the request (for example, `400`); never serve it unfiltered.
5. **Cluster-scoped objects** (no namespace) are never in the set: a restricted read serves none.
6. **`filtered` mode:** the extension's own delegated `SubjectAccessReview` for a cluster-wide read
   will deny — RBAC denied it in kube-apiserver already. Skipping that review is what makes the
   feature work, and is allowed only when rule 1 holds, only for `list`/`get`/`watch` of an opted-in
   resource, and only for the namespaces of the set. Everything else stays denied.
   **`narrowed` mode:** authorize as usual, then filter.
7. **Filter before Table conversion.** `kubectl`/`d8 k` request `as=Table`; filter in the storage
   (`List`/`Get`/`Watch` of the REST storage, or a predicate on `metadata.namespace`), not on the
   encoded response. Watch events must be filtered per event.
8. **`get` outside the set** answers `404 NotFound`, indistinguishable from a missing object, and does
   not name the scope or the set.

### Recommended implementation

The receiver library [`github.com/deckhouse/deckhouse/pkg/api-extension-sdk/scope`](../../../../pkg/api-extension-sdk/scope/README.md) implements this contract; an extension that cannot use it implements the check as follows.

**`k8s.io/apiserver`-based extension.** `WithAuthentication` removes the `X-Remote-*` headers after
authentication, so the check must run before it — as a handler filter placed outside
`WithAuthentication` in `Config.BuildHandlerChainFunc`:

- Build the verifier from `Config.Authentication.RequestHeaderConfig` the same way
  `DelegatingAuthenticatorConfig.New` builds the requestheader authenticator:
  `headerrequest.NewDynamicVerifyOptionsSecure(cfg.CAContentProvider.VerifyOptions, cfg.AllowedClientNames, cfg.UsernameHeaders, cfg.UIDHeaders, cfg.GroupHeaders, cfg.ExtraHeaderPrefixes)`.
  It wraps `x509request.NewDynamicCAVerifier`: the CA reloads with the ConfigMap, the common name is
  checked against the allowed names (any name is accepted when the list is empty), and
  authentication fails on an empty username header.
- Call `AuthenticateRequest` on a copy, `req.Clone(ctx)` (it deep-copies the headers): on success the
  header authenticator clears the `X-Remote-*` headers of the request it is given, and
  `WithAuthentication` further down still needs them.
- If `RequestHeaderConfig` is nil, or `VerifyOptions()` reports no CA, never honour the headers.
- On success, move the mode and the set into the request context. In both cases delete
  `X-Deckhouse-Scope-Mode` and `X-Deckhouse-Scope-Namespaces` from the request: `WithAuthentication`
  does not clear them, and code further down must read only the context value.
- Wrap the authorizer: for a request whose context carries `filtered`, allow `list`/`get`/`watch` of
  the opted-in resources without the delegated review; the storage filters by the set.

**Plain `net/http` server.** Request client certificates in `tls.Config` (`VerifyClientCertIfGiven` or
stricter). Verify `r.TLS.PeerCertificates[0]` with `leaf.Verify` against a pool built from the
requestheader CA, with `KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}` and the remaining
peer certificates as intermediates; check the common name against `requestheader-allowed-names`
(when not empty); require a non-empty username header. Reload the CA when the ConfigMap changes. TLS
must terminate in the process that runs the check: a TLS-terminating sidecar in front of the server
breaks the contract.

## Watch semantics

- The set sent with a watch is fixed for the life of the stream. A refresher in kube-aggregator
  recomputes it every refresh interval with the same inputs.
- The stream ends when the recomputed set differs from the one sent, when the read is no longer
  filtered or allowed (for example, the caller got cluster-wide access, or project scopes were
  disabled), when the `APIService` is deleted or no longer lists the resource in its annotation, or
  when recomputation keeps failing for the refresh grace period. Single failures keep the
  stream open with the old set.
- `timeoutSeconds` of a filtered watch is capped: a missing, zero, negative, unparsable or larger value
  is replaced by the cap.
- Ending the stream cancels the context of the proxied request. Before that, a plain HTTP watch in
  JSON or protobuf gets an `ERROR` event with a `410 Expired` Status: kube-aggregator forwards the
  response by whole frames only, drops a frame cut by the cancellation and appends the event in the
  encoding of the stream. A client-go reflector then lists again, so an informer cache gets the objects
  of the new set and drops those that left it. CBOR, other media types, compressed bodies, non-200
  answers, streams it cannot parse and frames over 16 MiB are forwarded as they come and only end,
  without the event; a 1xx answer before the 200 does not count.
- An upgraded (websocket) watch ignores the context after the hijack, so the refresher closes the
  hijacked client connection explicitly, without the event. A websocket client must list again after
  the connection closes; otherwise its cache keeps objects of namespaces that left the set and misses
  objects that already existed in namespaces that joined it.
- Revocation latency: roughly refresh interval + resolve cache TTL + propagation in
  `permission-browser-apiserver`; up to the refresh grace while recomputation fails.

Tunables are environment variables of the kube-apiserver process, read once at start; an invalid or
non-positive value falls back to the default with a warning. DKP does not set them.

| Variable | Default | Meaning |
|---|---|---|
| `SCOPEFILTER_WATCH_POLL_INTERVAL` | `1s` | Refresh interval of filtered watches, shared with watches served from storage |
| `SCOPEFILTER_RESOLVE_CACHE_TTL` | `1s` (capped at `30s`) | TTL of the shared cache of classification and RBAC floor answers |
| `SCOPEFILTER_PROXIED_WATCH_TIMEOUT` | `5m` | Cap of `timeoutSeconds` of a filtered watch of an aggregated API |
| `SCOPEFILTER_PROXIED_WATCH_REFRESH_GRACE` | `60s` | How long a filtered watch of an aggregated API survives failing recomputations |

## Threat model

- **Header spoofing.** kube-aggregator strips client copies of `X-Deckhouse-Scope-Mode` and
  `X-Deckhouse-Scope-Namespaces` from every proxied request, opted in or not. A copy is any header whose
  name matches ignoring case with `_` read as `-` (`x-deckhouse-scope-mode`,
  `X_Deckhouse_Scope_Mode`). The names are also removed from `Connection`: otherwise the reverse proxy
  would drop the value kube-apiserver set as hop-by-hop. Other `Connection` tokens, `Upgrade`
  included, stay.
- **Direct access to the Service** bypasses kube-apiserver. That is why the receiver must check the
  front-proxy certificate (rule 1), not the presence of headers.
- **The opt-in annotation is an admin-level grant.** Whoever can patch an `APIService` can open the
  data of that extension to scoped reads by users that RBAC denies — bounded by their RBAC floor, but
  served by an extension that may not implement the contract. In a default installation this is
  SuperAdmin and other `cluster-admin` subjects (including `system:sudouser` and platform components
  bound to `cluster-admin`, such as multitenancy-manager), the admin kubeconfig group
  `kubeadm:cluster-admins` (through `d8:control-plane-manager:admin-kubeconfig-supplement`), and the
  cert-manager cainjector ServiceAccount (`d8-cert-manager/cainjector`) when CA injection is enabled.
- **Redirecting `spec.service`** of an `APIService` to another backend is a vanilla Kubernetes risk
  with the same set of subjects; this patch does not address it.
- **`authorization.deckhouse.io` cannot opt in:** its `SelfScopedAccessReview` computes the RBAC floor,
  so a scoped read of it would compute its floor by calling itself.
- **Local `APIService`s cannot opt in:** they are served by kube-apiserver itself, never get the
  trusted headers, and a bypassed read that reaches them goes to the storage, which refuses it with
  `403` unless a storage decorator filters it.
- The registry the authorization filter consults and the per-version proxy may disagree for a moment
  while an `APIService` changes; both sides refuse what they do not filter, so the window fails
  closed.

## Observability

Metrics (subsystem `apiserver_scopefilter`, all ALPHA):

| Metric | Meaning |
|---|---|
| `apiserver_scopefilter_proxied_namespaces` | Histogram of the size of the set sent (buckets 0, 1, 10, 100, 1000, 10000) |
| `apiserver_scopefilter_proxied_watch_closed_total{reason}` | Watches ended by the refresher: `set_changed` (set differs, or the read is no longer filtered or allowed), `refresh_failed` (grace expired), `opt_in_withdrawn` (the `APIService` was deleted or no longer lists the resource) |
| `apiserver_scopefilter_proxied_header_limit_rejections_total` | Reads refused with `400` because of the header limit |
| `apiserver_scopefilter_watch_refresh_failures_total` | Failed recomputations; now also counts aggregated API watches |
| `apiserver_scopefilter_allowed_set_failures_total{operation,reason}` | New reason `no_resolvers` (filter wired without resolvers, `403`) |

Answers kube-aggregator writes itself are also recorded in `apiserver_request_terminations_total`.

Audit annotations:

| Key | Value |
|---|---|
| `authorization.k8s.io/decision` | `allow-scoped-proxied` for a bypassed read served through an aggregated API (or refused by kube-aggregator) |
| `authorization.k8s.io/reason` | `scope filtering mode, aggregated API` |
| `scope.deckhouse.io/proxied-mode` | `filtered` or `narrowed`; only when the request was forwarded with the headers |
| `scope.deckhouse.io/proxied-namespaces` | Number of namespaces in the set (a count, not the names); only when forwarded with the headers |

When a group has both a resource stored in kube-apiserver and the same resource opted in by another
version, the authorization filter leaves the decision to kube-aggregator, which records the path of the
version actually served.

## Testing an extension

[scope-aggregated-api-e2e/](scope-aggregated-api-e2e/README.md) holds a toy extension with its
manifests and `test/e2e.sh`, which automates most of the checks below. The toy is built on
`pkg/api-extension-sdk/scope`.

Run on a cluster with the patched kube-apiserver and the annotation on the `APIService`:

1. **Spoofed headers via the Service.** Call the extension Service directly (port-forward or a pod)
   with a bearer token of a user without RBAC and `X-Deckhouse-Scope-Mode: filtered`,
   `X-Deckhouse-Scope-Namespaces: <ns>`. The request must be authorized as usual (denied), not
   filtered. Repeat with a client certificate of another CA.
2. **Spoofed headers via kube-apiserver.** Send the same headers, also in `_`/lower-case spellings and
   listed in `Connection`, through kube-apiserver without `X-Deckhouse-Scope`. The extension must see
   no scope headers.
3. **Denied user with `--scope`.** `d8 k get <res> -A --scope=accessible --as=<user>` returns only
   objects of namespaces where the user has `list` on the resource; `--scope=system`/`projects`
   returns the intersection. A user with no access at all gets an empty list, not `403` and not
   everything.
4. **Table output.** The same commands with the default (Table) output and with `-o yaml` return the
   same objects.
5. **`get` outside the set.** `d8 k` rejects `--scope` without `-A`, so use the HTTP API:
   `curl -H "Authorization: Bearer $TOKEN" -H "X-Deckhouse-Scope: accessible" https://<kube-apiserver>/apis/<group>/<version>/namespaces/<ns>/<res>/<name>`.
   As a denied user with `<ns>` outside the set the answer is `404` from kube-apiserver. As an
   allowed user with `X-Deckhouse-Scope: system` and a non-system `<ns>` the answer must be `404`
   from the extension.
6. **Cluster-scoped objects** are never returned to a scoped read.
7. **Watch revocation.** Open `d8 k get <res> -A --scope=accessible -w --as=<user>`, then grant or
   revoke a RoleBinding in a namespace. The stream must end within a few seconds;
   `apiserver_scopefilter_proxied_watch_closed_total{reason="set_changed"}` grows. Check that a
   websocket watch ends too.

### Testing the patch

In the Kubernetes tree with the patch set applied (`GOFLAGS=-mod=readonly` is needed because the tree
uses `go.work` with a `vendor` directory that is not in sync):

```shell
export GOFLAGS=-mod=readonly
go test k8s.io/apiserver/pkg/registry/generic/scopefilter/... \
        k8s.io/apiserver/pkg/endpoints/filters/... \
        k8s.io/kube-aggregator/pkg/apiserver/...

./hack/install-etcd.sh   # once
PATH=$PWD/third_party/etcd:$PATH go test ./test/integration/scopefilter \
  -run TestScopeProxiedAggregatedAPI -count=1
```

| Test | Covers |
|---|---|
| `TestParseProxiedResources`, `TestProxiedRegistry`, `TestAllowedNamespaces`, `TestResolveError` | Annotation parsing, registry, set computation, error mapping |
| `TestWithAuthorization_ProxiedScopeBypass`, `TestWithAuthorization_ProxiedScopeAuditAnnotations` | Authorization bypass and audit decision |
| `TestProxyScope_*`, `TestStripTrustedScopeHeaders_Spellings`, `TestAPIServiceScopeLifecycle`, `TestAPIServiceScopeWithoutResolvers` | Proxy decision, header stripping, header limit, fail-closed paths, annotation lifecycle |
| `TestCapWatchTimeout`, `TestProxyScope_Watch*`, `TestProxyScope_WebsocketWatchSetChange`, `TestUpgradeAwareProxyIgnoresContext` | Watch cap, refresher, websocket close |
| `TestScopeWatchWriter*`, `TestProxyScope_ReflectorListsAgain`, `TestProxyScope_ErrorsReadByClientGo` | Frame-aware writer and the `410 Expired` event, relist of a real reflector, Status bodies read by client-go |
| `TestScopeProxiedAggregatedAPI` (integration) | End to end with a fake `permission-browser-apiserver` and a fake extension: front-proxy identity, filtered/narrowed sets, stripped client copies, annotation toggled on a live `APIService`, `get` outside the set, permission service failure, watch headers |

## Limitations and future work

- **Websocket watches** end without the `410 Expired` event (see [Watch semantics](#watch-semantics)).
- **Decision handle (option B2 of the design)** is not implemented. The set travels in headers and is
  bounded by the 1 MiB limit. In B2, kube-apiserver would send a short-lived unpredictable id of the
  decision, bound to the user, instead of the set; the extension would resolve it through an API (similar
  to TokenReview) and re-request it periodically. This removes the header size limit and the frozen set
  of a watch, but needs a new API and a store of decisions. The receiver library interface ("give me the
  namespace set of this request") is meant to keep a move from headers to B2 internal to the library.
- Only top-level `list`, `get` and `watch` are filtered; subresources and mutating verbs are not.
