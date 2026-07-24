---
title: Resource listing with access-based filtering
permalink: en/admin/configuration/access/authorization/filtered-listing.html
lang: en
search: scope, filtered listing, cross-namespace, accessible namespaces
description: How access-filtered resource listing across namespaces works in Deckhouse Kubernetes Platform.
---

In Kubernetes, the `kubectl get <resource> -A` command requires cluster-wide
permissions for the `list` operation. Otherwise, kube-apiserver returns
the `403 Forbidden` error, even if the user has access to dozens of namespaces
individually.

Deckhouse Kubernetes Platform (DKP) extends kube-apiserver so that a user without cluster-wide permissions can
explicitly request a response filtered down to what they can actually access,
instead of a refusal.

The mechanism is strictly opt-in.
For a client who does not request filtering, the behavior remains identical to the standard one used in Kubernetes.

## Using via Deckhouse CLI

In DKP, you can use the [Deckhouse CLI tool](../../cli/d8/) to work with the filtered listing mechanism.

### Resource filtering scopes

Using the `--scope` flag, you can define namespaces the resources will be returned from.

| Value | What is returned |
|---|---|
| `accessible` | Objects from every namespace where the user's RBAC allows the given operation on the given resource |
| `projects` | Objects from namespaces belonging to any [project](/modules/multitenancy-manager/cr.html#project) |
| `project:<name>` | Objects from the namespaces of one specific project |
| `system` | Objects from system namespaces only: `default` and namespaces whose names start with `d8-` or `kube-`. This is a fixed list of namespaces defined by their names and independent of projects. Other namespaces that neither belong to a project nor match the list of system namespaces are not returned when setting either `system` or `projects` scope |

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
- If the `d8 k get <resource> -A` command without `--scope` fails with `403 Forbidden`, Deckhouse CLI suggests suitable `--scope` variants.
- Shell completion works for the `--scope` flag values, including the names of
  existing projects (`--scope=project:<TAB>`).

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

Example of listing the Deployment objects of one project:

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
| The user has no cluster-wide permissions, and an invalid header value is submitted | `403 Forbidden` (fail-closed) |

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
     namespaces are actually available to the user for *this resource and this operation*. Computed via the `authorization.deckhouse.io` API
     (the [`user-authz`](/modules/user-authz/) module): the intersection of `AccessibleNamespaces` and
     `BulkSubjectAccessReview` results for the exact pair (`resource`, `verb`). Applied
     whenever RBAC denies the request.

   Only objects from namespaces in the final set make it into the response.

1. **Watch** (`-w`): Returns a filtered stream, and if the set of accessible
   namespaces changes "on the fly" (for example, permissions granted or revoked, or a namespace
   was moved to another project), it synthesizes `ADDED`/`DELETED` events for the
   affected objects without the client reconnecting.

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
- **Using the fail-closed model.** If the `authorization.deckhouse.io` API is unavailable
  (for example, if the `user-authz` module is disabled or `permission-browser` does not respond),
  the access boundary cannot be computed — a denied user gets the standard
  `403 Forbidden` response.
- **Indistinguishable refusals.** Requests like `project:<someone else's>` and
  `project:<nonexistent>` result in identical empty responses and project
  existence is not disclosed.

## Special case: namespaces

For the `namespaces` resource itself, filtering applies unconditionally,
regardless of used flags or headers. The `d8 k get namespaces` command returns only the
namespaces available to the user instead of `403 Forbidden`. Filtering of
arbitrary resources via `--scope` is a generalization of the same mechanism,
enabled explicitly.

## Components involved

| Component | Role |
|---|---|
| [Deckhouse CLI](../../cli/d8/)| Supports the `--scope` flag and transforms its values into headers |
| kube-apiserver (DKP patch) | Bypasses `403 Forbidden` and provides filtering at the storage layer |
| [`user-authz`](/modules/user-authz/) (`permission-browser`) | Defines the access boundary via `AccessibleNamespaces` and `BulkSubjectAccessReview` |
| [`multitenancy-manager`](/modules/multitenancy-manager/) | Provides namespace classification based on the `projects.deckhouse.io/project` label |
