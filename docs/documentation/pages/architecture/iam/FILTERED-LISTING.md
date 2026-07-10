---
title: Filtered resource listing
permalink: en/architecture/iam/filtered-listing.html
lang: en
search: scope, filtered listing, cross-namespace, accessible namespaces
description: How access-filtered cross-namespace resource listing works in Deckhouse Kubernetes Platform.
---

In vanilla Kubernetes, `kubectl get <resource> -A` requires cluster-wide
permissions for the `list` operation: without them, kube-apiserver returns
`403 Forbidden`, even if the user has access to dozens of namespaces
individually.

DKP extends kube-apiserver so that a user without cluster-wide permissions can
explicitly request a response filtered down to what they can actually access,
instead of a refusal. The mechanism is strictly opt-in: a client that does not
request filtering gets byte-for-byte vanilla behavior.

## --scope values

| Value | What is returned |
|---|---|
| `accessible` | Objects from every namespace where the user's RBAC allows the given operation on the given resource |
| `projects` | Objects from namespaces belonging to any [project](/modules/multitenancy-manager/cr.html#project) |
| `project:<name>` | Objects from the namespaces of one specific project |
| `system` | Objects from system namespaces (those not belonging to any project) |

## Using via d8

Use the `--scope` flag of `d8 k get` together with `-A/--all-namespaces`:

```shell
d8 k get pods -A --scope=accessible
d8 k get pods -A --scope=projects
d8 k get pods -A --scope=project:my-project
d8 k get configauditreports -A --scope=accessible   # works for custom resources too
d8 k get pods -A --scope=accessible -w              # watch is filtered as well
```

Details:

- Without `-A`, the flag is rejected with an error: filtering a single
  namespace's listing is meaningless.
- If `d8 k get <resource> -A` without `--scope` fails with `403 Forbidden`,
  the CLI suggests suitable `--scope` variants.
- Shell completion works for the flag values, including the names of
  existing projects (`--scope=project:<TAB>`).

## Using from your own code (HTTP API)

The CLI is just a transport: filtering is enabled by two HTTP request headers
sent to kube-apiserver. To use the mechanism from a backend service, a script,
or any other client, set the headers yourself.

| Header | Value | When required |
|---|---|---|
| `X-Deckhouse-Scope` | `accessible` \| `projects` \| `system` \| `project` | always (enables the mechanism) |
| `X-Deckhouse-Project` | project name | only together with `X-Deckhouse-Scope: project` |

The headers only affect reads (`list`, `get`, `watch`) of a top-level
namespaced resource without a subresource, on a cluster-scoped URL (a path
without `namespaces/<name>` — the one `-A` produces). In every other case
(mutating requests, subresources, cluster-scoped resources) they are ignored.

Example — list the pods available to a service account:

```shell
curl -H "Authorization: Bearer $TOKEN" \
     -H "X-Deckhouse-Scope: accessible" \
     https://<kube-apiserver>/api/v1/pods
```

Example — list the Deployment objects of one project:

```shell
curl -H "Authorization: Bearer $TOKEN" \
     -H "X-Deckhouse-Scope: project" \
     -H "X-Deckhouse-Project: my-project" \
     https://<kube-apiserver>/apis/apps/v1/deployments
```

Response semantics:

| Situation | Response |
|---|---|
| No headers | Vanilla behavior (`403 Forbidden` for a user without permissions) |
| Header present, RBAC denied the request | `200 OK` with objects from accessible namespaces (possibly an empty list) |
| Header present, the user has cluster-wide permissions | `200 OK`, narrowed by classification only (for example, `system` cuts off project namespaces) |
| Invalid header value from a user without permissions | `403 Forbidden` (fail-closed) |

In client-go, add the headers via `rest.Config.WrapTransport`.

## How it works

1. **kube-apiserver authorization filter.** If RBAC denied the request, the
   filter — instead of an immediate `403 Forbidden` — checks three conditions:
   the request is a read (`list`/`get`/`watch`) of a top-level resource
   without a subresource; the header is present; and a filtering decorator is
   registered for this (group, resource). Only when all three hold is the
   request passed on, marked "filter at the storage layer".

1. **The storage layer** computes the final namespace set as the intersection
   of two independent sets:

   - **Classification** — which namespaces match the `--scope` value.
     Determined by the `projects.deckhouse.io/project` label that
     multitenancy-manager puts on project namespaces. Resolved by a direct
     request to kube-apiserver and does not depend on the user.
   - **Access boundary** (RBAC floor in the mechanism's code) — which
     namespaces are actually available to the user for *this resource and
     this operation*. Computed via the `authorization.deckhouse.io` API
     (the user-authz module): the intersection of `AccessibleNamespaces` and
     `BulkSubjectAccessReview` for the exact (resource, verb) pair. Applied
     whenever RBAC denied the request.

   Only objects from namespaces in the final set make it into the response.

1. **Watch** (`-w`) returns a filtered stream, and if the set of accessible
   namespaces changes on the fly (permissions granted or revoked, a namespace
   changed its project), it synthesizes `ADDED`/`DELETED` events for the
   affected objects without the client reconnecting.

All built-in namespaced resources and namespaced CRD resources are covered.

## Security model

- **No header — vanilla behavior.** Controllers and operators relying on
  `403 Forbidden` as a signal do not break.
- **The header does not widen access.** The access boundary is computed on
  the server from the request identity; a client that crafts the header by
  hand sees no more than its RBAC allows. The boundary is narrow — per exact
  (resource, verb) pair: `get secrets` permission in a namespace does not
  expose pods in it.
- **Fail-closed.** If the `authorization.deckhouse.io` API is unavailable
  (the user-authz module is disabled or permission-browser does not respond),
  the access boundary cannot be computed — a denied user gets the usual
  `403 Forbidden`.
- **Indistinguishable refusals.** Requests for `project:<someone else's>` and
  `project:<nonexistent>` produce identical empty responses — project
  existence is not disclosed.

## Special case: namespaces

For the `namespaces` resource itself, filtering applies unconditionally,
without any flag or headers: `d8 k get namespaces` returns only the
namespaces available to the user instead of `403 Forbidden`. Filtering of
arbitrary resources via `--scope` is a generalization of the same mechanism,
enabled explicitly.

## Components involved

| Component | Role |
|---|---|
| [Deckhouse CLI](../../cli/d8/)| The `--scope` flag, translation into headers |
| kube-apiserver (DKP patch) | `403 Forbidden` bypass + filtering at the storage layer |
| [`user-authz`](/modules/user-authz/) (permission-browser) | `AccessibleNamespaces`, `BulkSubjectAccessReview` — the access boundary |
| [`multitenancy-manager`](/modules/multitenancy-manager/) | The `projects.deckhouse.io/project` label — namespace classification |
