---
title: User-authz module
permalink: en/architecture/iam/user-authz.html
search: user-authz, RBAC, role-based access control, authorization
description: Architecture of the user-authz module in Deckhouse Kubernetes Platform.
---

The [`user-authz`](/modules/user-authz/) module implements role-based access control (RBAC) in Deckhouse Kubernetes Platform (DKP). The module creates cluster roles for managing user and group access to cluster resources, and, in the BE, SE, SE+, EE, CSE Lite, and CSE Pro editions of DKP, also supports namespace-scoped authorization ([multitenancy](./multitenancy.html)).

The module works with the following custom resources of the `deckhouse.io` API group:

- [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule): Defines access rules at the cluster level.
- [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule): Defines access rules within a single namespace.

In the BE, SE, SE+, EE, CSE Lite, and CSE Pro editions of DKP, the module additionally works with the following custom resources of the `authorization.deckhouse.io` API group:

- BulkSubjectAccessReview: A resource for bulk-checking user permissions across multiple objects or actions.
- AccessibleNamespace: A resource for retrieving the list of namespaces accessible to a specific user.

For more details about module configuration and usage examples, refer to the [corresponding documentation section](/modules/user-authz/).

## Module architecture

{% alert level="info" %}
The following assumptions are made to simplify the diagram:

* The diagram shows direct interaction between containers from different pods. In practice, they communicate through corresponding Kubernetes Services (internal load balancers). Service names are omitted when they are clear from context. In other cases, the service name is shown above the arrow.
* Pods can run with multiple replicas, but all pods are shown as a single replica in the diagram.
{% endalert %}

The level 2 C4 architecture of the [`user-authz`](/modules/user-authz/) module and its interactions with other DKP components are shown in the following diagram:

![User-authz module architecture](../../images/architecture/iam/c4-l2-user-authz.svg)

## Module components

The module consists of the following components:

1. **User-authz-controller** (Deployment): This component includes a single **user-authz-controller** container and watches the ClusterAuthorizationRule and AuthorizationRule custom resources, creating, updating, and deleting the corresponding ClusterRoleBinding and RoleBinding resources.

The BE, SE, SE+, EE, CSE Lite, and CSE Pro editions of DKP additionally include the following components:

1. **User-authz-webhook** (DaemonSet): An optional component that implements the [Webhook authorization mode](https://kubernetes.io/docs/reference/access-authn-authz/webhook/) for kube-apiserver. On each `SubjectAccessReview` request, the component checks the namespace access restrictions defined in the ClusterAuthorizationRule custom resource ([multitenancy](./multitenancy.html)), and delegates decisions for other requests to standard RBAC. The component runs on all master nodes with `hostNetwork` enabled.

   The Deckhouse controller deploys this component if the [`.enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) parameter in the module settings is set to `true` (default is `false`).

   The webhook authorization mode for kube-apiserver is configured by the [`control-plane-manager`](/modules/control-plane-manager/) module if the [`.controlPlaneConfigurator.enabled`](/modules/user-authz/configuration.html#parameters-controlplaneconfigurator-enabled) parameter in the module settings is set to `true` (the default).

   Includes the following containers:

   - **user-authz-webhook**: Main container.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to component metrics.

1. **Permission-browser-apiserver** (Deployment): An optional component, a [Kubernetes API extension](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/) that publishes the `authorization.deckhouse.io` API group with the BulkSubjectAccessReview and AccessibleNamespace resources.

   The BulkSubjectAccessReview and AccessibleNamespace custom resources are used by the [`console`](/modules/console/) module for bulk permission checks and for retrieving the list of namespaces accessible to a user.

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
