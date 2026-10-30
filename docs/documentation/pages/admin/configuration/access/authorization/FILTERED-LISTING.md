---
title: Resource listing with access-based filtering
permalink: en/admin/configuration/access/authorization/filtered-listing.html
lang: en
search: scope, filtered listing, cross-namespace, accessible namespaces
description: How access-filtered resource listing across namespaces works in Deckhouse Platform.
---

In Kubernetes, the `kubectl get <resource> -A` command requires cluster-wide
permissions for the `list` operation. Otherwise, kube-apiserver returns
the `403 Forbidden` error, even if the user has access to dozens of namespaces
individually.

Deckhouse Platform (DP) extends kube-apiserver so that a user without cluster-wide permissions can
explicitly request a response filtered down to what they can actually access,
instead of a refusal.

The mechanism is strictly opt-in.
For a client who does not request filtering, the behavior remains identical to the standard one used in Kubernetes.

## Using via Deckhouse CLI

In DP, you can use the [Deckhouse CLI tool](../../../../cli/d8/) to work with the filtered listing mechanism.

### Resource filtering scopes

Using the `--scope` flag, you can define namespaces the resources will be returned from.

| Value | What is returned |
|---|---|
| `accessible` | Objects from every namespace where the user's RBAC allows the given operation on the given resource |
| `projects` | Objects from namespaces belonging to any [project](/modules/multitenancy-manager/cr.html#project), that is, namespaces with a non-empty value of the `projects.deckhouse.io/project` label |
| `project:<name>` | Objects from the namespaces of one specific [project](../../../../glossary.html#project). The name must be a valid project name |
| `system` | Objects from [system namespaces](../../../../glossary.html#system-namespace) only: `default` and namespaces whose names start with `d8-` or `kube-`. This is a fixed list of namespaces defined by their names and independent of projects. Other namespaces that neither belong to a project nor match the list of system namespaces are not returned when setting either `system` or `projects` scope |

The `projects` and `project:<name>` scopes require the [`multitenancy-manager`](/modules/multitenancy-manager/) module.
While the module is disabled, requests with these scopes are rejected with the `400 Bad Request` error.

### Examples

Use the `--scope` flag in the `d8 k get` command together with the `-A` flag (or `--all-namespaces`).

Obtaining objects from all namespaces available to the user:

```shell
d8 k get pods -A --scope=accessible
```

Obtaining objects from project namespaces only:

```shell
d8 k get pods -A --scope=projects
```

Obtaining objects from a specified project:

```shell
d8 k get pods -A --scope=project:my-project
```

The mechanism is applicable to custom resources as well:

```shell
d8 k get configauditreports -A --scope=accessible
```

The filtering can be applied to watch requests as well:

```shell
d8 k get pods -A --scope=accessible -w
```

Details:

- Using `--scope` without the `-A` flag is rejected with an error because filtering a single namespace's listing is meaningless.
- If the `d8 k get <resource> -A` command without `--scope` fails with `403 Forbidden`, [Deckhouse CLI](../../../../glossary.html#deckhouse-cli) suggests suitable `--scope` variants.
- Shell completion works for the `--scope` flag values, including the names of
  existing projects (`--scope=project:<TAB>`).

#### Example of an interface query for where the user may perform actions

With the multitenancy mode enabled ([`enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy)), you can use the SelfScopedAccessReview resource to request information about the namespaces in which the user is allowed to perform actions. The query gives the user who creates it information on where actions are allowed for them within a [filtering scope](#resource-filtering-scopes). One query replaces an access check per namespace. The namespaces are counted by the rule that filters a list sent with the same scope, so a section built on the answer opens exactly when its list has objects to return.

Any authenticated user can create the resource, and it answers only about the caller. A query with the `spec.user`, `spec.groups`, `spec.uid` or `spec.extra` fields is refused.

Example query from a Deckhouse web interface page that shows deployments of all projects, offers to create one and shows pod logs:

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: SelfScopedAccessReview
spec:
  scope: projects
  requests:
  - group: apps
    resource: deployments
    verbs: [list, watch]
  - group: apps
    resource: deployments
    verbs: [create]
    includeNamespaces: true
  - group: ""
    resource: pods
    subresource: log
    verbs: [get]
    includeNamespaces: true
EOF
```

Example answer for a user who administers the `shop-dev` and `shop-stage` namespaces of a project and has read access without pod logs in `shop-prod`:

```yaml
status:
  results:
  - allowed: true
    cluster: false
    count: 3
  - allowed: true
    cluster: false
    count: 2
    namespaces: [shop-dev, shop-stage]
  - allowed: true
    cluster: false
    count: 2
    namespaces: [shop-dev, shop-stage]
```

Example answer for a cluster administrator. The namespaces are neither counted nor listed, even with `includeNamespaces: true`:

```yaml
status:
  results:
  - allowed: true
    cluster: true
  - allowed: true
    cluster: true
  - allowed: true
    cluster: true
```

Example answer for a user who can only read deployments in the `billing-prod` namespace of another project:

```yaml
status:
  results:
  - allowed: true
    cluster: false
    count: 1
  - allowed: false
    cluster: false
    count: 0
    namespaces: []
  - allowed: false
    cluster: false
    count: 0
    namespaces: []
```

For details on the SelfScopedAccessReview resource (query), including how it works, the query and answer fields, errors and usage recommendations, refer to the [`user-authz` module documentation](/modules/user-authz/faq.html#how-does-an-interface-find-out-where-a-user-may-perform-an-action).

## Using from your own code (HTTP API)

The CLI is just a transport mechanism. The filtering is enabled by two HTTP request headers
sent to kube-apiserver. To use the mechanism from you own service, a script,
or any other client, set the headers yourself.

| Header | Value | When required |
|---|---|---|
| `X-Deckhouse-Scope` | `accessible` \| `projects` \| `system` \| `project` | Always (enables the mechanism) |
| `X-Deckhouse-Project` | Project name | Only together with `X-Deckhouse-Scope: project` |

The headers only affect read operations (`list`, `get`, `watch`) of a top-level
namespaced resources without a subresource, on a cluster-scoped URL (a path
without `namespaces/<name>`, which is produced when the `-A` flag is used).

In every other case
(mutating requests, requests to subresources and cluster-scoped resources) headers are ignored.

Example of listing the pods available to a service account:

```shell
curl -H "Authorization: Bearer $TOKEN" \
     -H "X-Deckhouse-Scope: accessible" \
     https://<kube-apiserver>/api/v1/pods
```

Example of listing the Deployment objects of one [project](../../../../glossary.html#project):

```shell
curl -H "Authorization: Bearer $TOKEN" \
     -H "X-Deckhouse-Scope: project" \
     -H "X-Deckhouse-Project: my-project" \
     https://<kube-apiserver>/apis/apps/v1/deployments
```

Response semantics:

| Situation | Response |
|---|---|
| No headers | Standard Kubernetes behavior (the `403 Forbidden` error for a user without permissions) |
| Header present but RBAC denied the request | `200 OK` with a list of objects from accessible namespaces (the list can be empty) |
| Header present, and the user has cluster-wide permissions | `200 OK`. The filtering is applied by namespace classification only (for example, `system` keeps only `default`, `d8-*`, and `kube-*` namespaces) |
| Invalid header value: an unknown scope, `X-Deckhouse-Project` without `X-Deckhouse-Scope: project`, or a project name that is not a valid name | `400 Bad Request` for every user, before the permissions are checked |
| The `projects` or `project` scope while the `multitenancy-manager` module is disabled | `400 Bad Request` |
| `get` of an object outside the resulting set of namespaces | `404 Not Found`, as for an object that does not exist |
| RBAC denied the request, and `permission-browser` cannot answer now: it does not answer in time, is overloaded, or is unreachable | `503 Service Unavailable` with the `Retry-After` header; no objects are returned |
| RBAC denied the request, and the access boundary cannot be computed for another reason | `403 Forbidden` (fail-closed) |

When using client-go, add the headers via `rest.Config.WrapTransport`.

## How it works

1. **Kube-apiserver authorization filter.**

   If RBAC denied the request, the
   filter — instead of returning `403 Forbidden` immediately — checks three conditions:

   - The read operation (`list`, `get`, or `watch`) is underway.
   - The request is addressed to the main resource and not the subresource.
   - A filtering decorator is registered for this resource and the header is present.

   Only when all three conditions are met, the request is passed on, with a mark of filtering required at the storage layer.

1. **The storage layer** computes the final namespace set as the intersection
   of two independent sets:

   - **Classification**: Which namespaces match the `--scope` value.
     Determined by the `projects.deckhouse.io/project` label that
     the [`multitenancy-manager`](/modules/multitenancy-manager/) module assigns to project namespaces. Resolved by a direct
     request to kube-apiserver and does not depend on the user.
   - **Access boundary** (with RBAC floor in the mechanism's code): Which
     namespaces are actually available to the user for *this resource and this operation*. Computed with one `SelfScopedAccessReview`
     request to the `authorization.deckhouse.io` API (the [`user-authz`](/modules/user-authz/) module) for the exact
     resource, subresource, and verb, the same answer the web interface uses to decide what to show. Applied
     whenever RBAC denies the request. The boundary also includes the namespaces where a RoleBinding grants
     permissions to every authenticated user (the `system:authenticated` group), although the namespace list
     does not show them.

   Only objects from namespaces in the final set make it into the response.

1. **Watch** (`-w`): Returns a filtered stream, and if the set of accessible
   namespaces changes "on the fly" (for example, permissions granted or revoked, or a namespace
   was moved to another project), it synthesizes `ADDED`/`DELETED` events for the
   affected objects without the client reconnecting.

   The set is refreshed every second. A synthesized `DELETED` event carries the last `resourceVersion` of the stream.
   If the set cannot be refreshed (for example, `permission-browser` does not respond), the stream keeps the previous set for up to 60 seconds, then sends an error and closes.
   The client reconnects and gets `503 Service Unavailable` while `permission-browser` cannot answer, or `403 Forbidden` while the set cannot be computed for another reason.
   If the `multitenancy-manager` module is disabled while a watch with the `projects` or `project` scope is open, the stream sends a `400 Bad Request` error and closes at once.
   If `permission-browser` answers that the user may now perform the request in the whole cluster, the stream sends `410 Gone` and closes, and the client lists again.

The mechanism supports all built-in namespaced Kubernetes resources and namespaced resources defined via CRD.

## Security model

- **Standard Kubernetes behavior without headers.** Controllers and operators relying on
  `403 Forbidden` as a signal continue normal operation.
- **Headers do not widen access permissions.** The access boundary is computed on
  the server based on the identity of a user who made the request.
  Even if a client inserts the headers
  manually they can't access objects out of what their RBAC allows. The check is made for each
  pair (`resource`, `verb`). Therefore, for example, a user with the `get secrets` permission in a namespace can't
  access pods in it.
- **Using the fail-closed model.** If the `authorization.deckhouse.io` API is not served
  (for example, if the `user-authz` module is disabled, or `permission-browser` does not serve
  `SelfScopedAccessReview` yet during an update), the access boundary cannot be computed — a denied user gets the standard
  `403 Forbidden` response. The same response is returned when `permission-browser` allows the operation in the whole cluster
  while kube-apiserver has denied it: the mechanism never answers with all namespaces.
  If `permission-browser` cannot answer now (it does not answer in time, answers `429 Too Many Requests` or a `5xx` error,
  or is unreachable), a scoped request gets `503 Service Unavailable` with the `Retry-After` header instead: the access is
  unknown rather than denied, and the client may retry. Neither response returns any objects.
- **Indistinguishable refusals.** Requests like `project:<someone else's>` and
  `project:<nonexistent>` result in identical empty responses and [project](../../../../glossary.html#project)
  existence is not disclosed.

## Special case: namespaces

For the `namespaces` resource itself, filtering applies unconditionally,
regardless of used flags or headers. The `d8 k get namespaces` command returns only the
namespaces available to the user instead of `403 Forbidden`. Filtering of
arbitrary resources via `--scope` is a generalization of the same mechanism,
enabled explicitly.

If the `console` module older than 1.44.0 is enabled, the namespace list is not filtered: the access levels of ClusterAuthorizationRule and AuthorizationRule and the namespace roles keep the permission to list every namespace.

The same list is returned by the read-only accessiblenamespaces resource, which any authenticated user can request for themselves:

```bash
d8 k get accessiblenamespaces
```

This is handy for scripts and interfaces: there is no need to iterate over namespaces and check access to each one.

## Special case: projects

A `Project` is cluster-scoped, but its visibility follows the same namespace
boundary: a user sees a project when they can see any of its namespaces — the
main one or an additional one. A namespace belongs to a project only when its
`projects.deckhouse.io/project` label has a non-empty value; the project is never
derived from the namespace name. A namespace without the label shows the virtual project
that `multitenancy-manager` counts it in: `deckhouse` for `d8-*`, `kube-*`, `upmeter-*`
and namespaces with the `heritage: deckhouse` or `heritage: upmeter` label, `default` for the
rest. A user who may list namespaces in the whole cluster sees every project, both
virtual ones included. Filtering applies unconditionally here as well,
so `d8 k get projects` returns the accessible subset instead of
`403 Forbidden`, and a `get` of a project unavailable for the user answers `NotFound`.

Because of that, access levels granted through a `ClusterAuthorizationRule` no
longer carry a cluster-wide read of `projects`: the grant would let RBAC allow
the listing before the filter could narrow it. As a result,
`d8 k auth can-i list projects` answers `no` for such a user while the listing
itself works and returns their projects. Writing projects is unaffected.

## Monitoring

kube-apiserver exports the following metrics of the mechanism:

| Metric | Meaning |
|---|---|
| `apiserver_scopefilter_allowed_set_failures_total` | Requests answered with an error because the access boundary could not be computed, by the `operation` label (`list`, `get`, `watch`) and the `reason` label: `unavailable` (`permission-browser` cannot answer now, the request got `503 Service Unavailable`) or `denied` (any other failure, the request got `403 Forbidden`) |
| `apiserver_scopefilter_cluster_disagreements_total` | Requests kube-apiserver denied while `permission-browser` allowed them in the whole cluster, answered with `403 Forbidden` |
| `apiserver_scopefilter_watch_refresh_failures_total` | Failed refreshes of the namespace set of an open watch, timeouts included |
| `apiserver_scopefilter_watch_closed_total` | Watches closed by the mechanism, by the `reason` label: `refresh_failures` (the set could not be refreshed for 60 seconds), `overflow` (the client did not read the events in time), `multitenancy_disabled` (the `projects` and `project` scopes are no longer served) or `access_changed` (the user may now perform the request in the whole cluster) |

## Components involved

| Component | Role |
|---|---|
| [Deckhouse CLI](../../../../cli/d8/)| Supports the `--scope` flag and transforms its values into headers |
| kube-apiserver (DP patch) | Bypasses `403 Forbidden` and provides filtering at the storage layer |
| [`user-authz`](/modules/user-authz/) (`permission-browser`) | Defines the access boundary via `SelfScopedAccessReview` |
| [`multitenancy-manager`](/modules/multitenancy-manager/) | Provides namespace classification based on the `projects.deckhouse.io/project` label |
| [`user-authz`](/modules/user-authz/) (the `d8-user-authz-multitenancy-state` ConfigMap) | Tells kube-apiserver whether the `multitenancy-manager` module is enabled, in every edition |
