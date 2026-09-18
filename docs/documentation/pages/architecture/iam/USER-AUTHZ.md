---
title: User-authz module
permalink: en/architecture/iam/user-authz.html
search: user-authz, RBAC, role-based access control, authorization
description: Architecture of the user-authz module in Deckhouse Platform.
---

The [`user-authz`](/modules/user-authz/) module implements role-based access control (RBAC) in Deckhouse Platform (DP). The module creates cluster roles for managing user and group access to cluster resources, and, in the Ultimate, CSE Core, and CSE Pro editions of DP, also supports namespace-scoped authorization ([multitenancy](./multitenancy.html)).

The module works with the following custom resources of the `deckhouse.io` API group:

- [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule): Defines access rules at the cluster level.
- [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule): Defines access rules within a single namespace.

In the Ultimate, CSE Core, and CSE Pro editions of DP, the module additionally works with the following custom resources of the `authorization.deckhouse.io` API group:

- BulkSubjectAccessReview: A resource for bulk-checking user permissions across multiple objects or actions.
- AccessibleNamespace: A resource for retrieving the list of namespaces accessible to a specific user.
- WhoCan: A resource for finding which users, groups, and service accounts are allowed to perform a given action.
- SubjectAccessReport: A resource for retrieving the full list of permissions granted to a subject.
- RoleAccessReport: A resource for retrieving the full list of resources and actions granted by a role.

For more details about module configuration and usage examples, refer to the [corresponding documentation section](/modules/user-authz/).

## Module architecture

{% alert level="info" %}
The following assumptions are made to simplify the diagram:

* The diagram shows direct interaction between containers from different pods. In practice, they communicate through corresponding Kubernetes Services (internal load balancers). Service names are omitted when they are clear from context. In other cases, the service name is shown above the arrow.
* Pods can run with multiple replicas, but all pods are shown as a single replica in the diagram.
{% endalert %}

The level 2 C4 architecture of the [`user-authz`](/modules/user-authz/) module and its interactions with other DP components are shown in the following diagram:

![User-authz module architecture](../../images/architecture/iam/c4-l2-user-authz.svg)

## Module components

The module consists of the following components:

1. **User-authz-controller** (Deployment): This component includes a single **user-authz-controller** container and watches the ClusterAuthorizationRule and AuthorizationRule custom resources, creating, updating, and deleting the corresponding ClusterRoleBinding and RoleBinding resources.

The Ultimate, CSE Core, and CSE Pro editions of DP additionally include the following components:

1. **User-authz-webhook** (DaemonSet): An optional component that implements the [Webhook authorization mode](https://kubernetes.io/docs/reference/access-authn-authz/webhook/) for kube-apiserver, placed between the built-in Node and RBAC authorizers in the authorization chain. The component runs on all master nodes with `hostNetwork` enabled.

   The webhook authorization mode for kube-apiserver is configured by the [`control-plane-manager`](/modules/control-plane-manager/) module if the [`.controlPlaneConfigurator.enabled`](/modules/user-authz/configuration.html#parameters-controlplaneconfigurator-enabled) parameter in the module settings is set to `true` (the default). As part of this, the module creates an AuthorizationConfiguration whose `matchConditions` parameter excludes the following subjects from being checked by the webhook:

   - Core control plane system identities, such as `kubernetes-admin`.
   - Node identities, `system:node:*`.
   - Service accounts from the `kube-system` and `d8-*` namespaces.

   Upon receiving a `SubjectAccessReview` request, the webhook checks the namespace access restrictions defined in the ClusterAuthorizationRule custom resource ([multitenancy](./multitenancy.html)).

   The component can only deny a request explicitly; for every other request it returns no opinion, and the decision falls through to the RBAC authorizer. User-authz-webhook is fail-closed: `failurePolicy` is set to `Deny` with a 3-second timeout, so if the webhook is unavailable or does not respond in time, kube-apiserver denies every non-excluded request instead of falling back to RBAC.

   The Deckhouse controller deploys this component if the [`.enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) parameter in the module settings is set to `true` (default is `false`).

   Includes the following containers:

   - **user-authz-webhook**: Main container.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to component metrics.

1. **Permission-browser-apiserver** (Deployment): An optional component, a [Kubernetes API extension](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/) that publishes custom resources of the `authorization.deckhouse.io` API group.

   The BulkSubjectAccessReview, AccessibleNamespace, WhoCan, SubjectAccessReport, and RoleAccessReport custom resources of the `authorization.deckhouse.io` API group are used by the [`console`](/modules/console/) module and provide extended capabilities for determining user, resource, or action permissions.

   The Deckhouse controller deploys this component if the [`.enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) parameter in the module settings is set to `true` (default is `false`).

   Includes the following containers:

   - **permission-browser-apiserver**: Main container.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to component metrics.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   - Watches the ClusterAuthorizationRule and AuthorizationRule custom resources.
   - Manages the ClusterRoleBinding and RoleBinding resources.
   - Reads Namespace, RBAC resources, and the discovery API.
   - Authorizes requests for metrics.

The following external components interact with the module:

1. **Kube-apiserver**:

   - Authorizes requests to cluster resources.
   - Forwards requests to the `authorization.deckhouse.io` API group.

1. **Prometheus-main**: Collects metrics from the user-authz-webhook and permission-browser-apiserver components.
