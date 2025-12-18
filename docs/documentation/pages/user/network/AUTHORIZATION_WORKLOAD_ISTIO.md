---
title: "Managing authorization and access to workload with Istio"
permalink: en/user/network/authorization-workload-istio.html
---

You can use the [istio](/modules/istio/) module to manage authorization and control access to workloads.
Before configuring authorization, make sure the module is enabled in the cluster.

## Authorization

Authorization is managed using the [AuthorizationPolicy](#authorizationpolicy-resource) resource from Istio.
When this resource is created for a Service, the following request decision rules apply:

- If a request matches a `DENY` policy — deny the request.
- If there are no `ALLOW` policies for the Service — allow the request.
- If a request matches an `ALLOW` policy — allow the request.
- All other requests — to be denied.

In other words, if something is explicitly denied, the deny rule takes precedence.
If something is explicitly allowed, only explicitly permitted requests are allowed (denies still have priority).

You can use the following arguments when writing authorization rules:

- Service identifiers and wildcards based on them (`mycluster.local/ns/myns/sa/myapp` or `mycluster.local/*`)
- Namespace
- IP ranges
- HTTP headers
- JWT tokens from application requests

## AuthorizationPolicy resource

For more details on AuthorizationPolicy, refer to the [Istio documentation](https://istio.io/v1.19/docs/reference/config/security/authorization-policy/).

The AuthorizationPolicy resource enables and defines access control to workloads.
It supports both ALLOW and DENY rules described above.

Arguments for making authorization decisions:

- `source`:
  - `namespace`
  - `principal` (user identifier obtained after authentication)
  - IP
- `destination`:
  - `method` (`GET`, `POST`, etc.)
  - `host`
  - `port`
  - URI
- [`conditions`](https://istio.io/v1.19/docs/reference/config/security/conditions/#supported-conditions):
  - HTTP headers
  - `source` arguments
  - `destination` arguments
  - JWT tokens
