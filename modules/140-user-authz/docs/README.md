---
title: "The user-authz module"
description: "Authorization and role-based access control to the resources of the Deckhouse Kubernetes Platform cluster."
---

The module generates role-based access model objects based on the standard Kubernetes RBAC mechanism. The module creates a set of cluster roles (`ClusterRole`) suitable for most user and group access management tasks.

{% alert level="warning" %}
Starting from Deckhouse Kubernetes Platform v1.64, the module features a experimental role-based access model. The current role-based access model will continue to operate but support for it will be discontinued in the future.

The experimental role-based access model is incompatible with the current one.
{% endalert %}

The module implements a role-based access model based on the standard RBAC Kubernetes mechanism. It creates a set of cluster roles (`ClusterRole`) suitable for most user and group access management tasks.

<div style="height: 0;" id="the-new-role-based-model"></div>

## Experimental role-based model

Unlike the [current DKP role-based model](#current-role-based-model), the new role-based one does not use `ClusterAuthorizationRule` and `AuthorizationRule` resources. All access rights are configured in the standard Kubernetes RBAC way, i.e., by creating `RoleBinding` or `ClusterRoleBinding` resources and specifying one of the roles prepared by the `user-authz` module in them.

The module creates special aggregated cluster roles (`ClusterRole`). By using these roles in `RoleBinding` or `ClusterRoleBinding`, you can do the following:
- Manage access to modules of a specific [subsystem](#subsystems-of-the-role-based-model).

  For example, you can use the `d8:manage:networking:manager` role in `ClusterRoleBinding` to allow a network administrator to configure *network* modules (such as `cni-cilium`, `ingress-nginx`, `istio`, etc.).
- Manage access to *user* resources of modules within the namespace.

  For example, the `d8:use:role:manager` role in `RoleBinding` enables deleting/creating/editing the [PodLoggingConfig](../log-shipper/cr.html#podloggingconfig) resource in the namespace. At the same time, it does not grant access to the cluster-wide [ClusterLoggingConfig](../log-shipper/cr.html#clusterloggingconfig) and [ClusterLogDestination](../log-shipper/cr.html#clusterlogdestination) resources of the `log-shipper` module, nor does it allow configuration of the `log-shipper` module itself.

The roles created by the module are divided into two classes:
- [Use roles](#use-roles) — for assigning rights to users (such as application developers) **in a specific namespace**.
- [Manage roles](#manage-roles) — for assigning rights to administrators.

### Use roles

{% alert level="warning" %}
The use role can only be used in the `RoleBinding` resource.
{% endalert %}

Use roles are intended to assign rights to a user **in a specific namespace**. Users refer to, for example, developers who use a cluster configured by an administrator to deploy their applications. Such users don't need to manage DKP modules or a cluster, but they need to be able to, for example, create their Ingress resources, configure application authentication, and collect logs from applications.

The use role defines permissions for accessing namespaced resources of modules and standard namespaced resources of Kubernetes (`Pod`, `Deployment`, `Secret`, `ConfigMap`, etc.).

The module creates the following use roles:
- `d8:use:role:viewer` — allows viewing standard Kubernetes resources in a specific namespace, except for Secrets and RBAC resources, as well as authenticating in the cluster;
- `d8:use:role:user` — in addition to the role `d8:use:role:viewer` it allows viewing secrets and RBAC resources in a specific namespace, connecting to pods, deleting pods (but not creating or modifying them), executing `kubectl port-forward` and `kubectl proxy`, as well as changing the number of replicas of controllers;
- `d8:use:role:manager` — in addition to the role `d8:use:role:user` it allows managing module resources (for example, `Certificate`, `PodLoggingConfig`, etc.) and standard namespaced Kubernetes resources (`Pod`, `ConfigMap`, `CronJob`, etc.) in a specific namespace;
- `d8:use:role:admin` — in addition to the role `d8:use:role:manager` it allows managing the resources `ResourceQuota`, `ServiceAccount`, `Role`, `RoleBinding`, `NetworkPolicy` in a specific namespace.

### Manage roles

{% alert level="warning" %}
The manage role does not grant access to the namespace of user applications.

The manage role grants access only to system namespaces (starting with `d8-` or `kube-`), and only to those system namespaces where the modules of the corresponding role subsystem are running.
{% endalert %}

Manage roles are intended for assigning rights to manage the entire platform or a part of it (the [subsystem](#subsystems-of-the-role-based-model)), but not the users applications themselves. The manage role, for example, can allow a security administrator to manage security modules (responsible for the security functions of the cluster). Thus, the security administrator will be able to configure authentication, authorization, security policies, etc., but will not be able to manage other cluster functions (such as network and monitoring settings) or change settings in the namespaces of users applications.

The manage role defines access rights:
- to cluster-wide Kubernetes resources;
- to manage DKP modules (`moduleConfig` resource) within the [subsystem](#subsystems-of-the-role-based-model) of the role, or to all DKP modules for the role `d8:manage:all:*`;
- to manage cluster-wide resources of DKP modules within the [subsystem](#subsystems-of-the-role-based-model) of the role, or to all resources of DKP modules for the role `d8:manage:all:*`;
- to system namespaces (starting with `d8-` or `kube-`) in which the modules of the [subsystem](#subsystems-of-the-role-based-model) of the role operate, or to all system namespaces for the role `d8:manage:all:*`.

The manage role name format is `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>`, where:
- `SUBSYSTEM` is the role's subsystem. It can be one of the [subsystem](#subsystems-of-the-role-based-model), or `all`, for access across all subsystems;
- `ACCESS_LEVEL` is the access level.

  Examples of manage roles:
  - `d8:manage:all:viewer` — access to view the configuration of all DKP modules (`moduleConfig` resource), their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except Secrets and RBAC resources) in all system namespaces (starting with `d8-` or `kube-`);
  - `d8:manage:all:manager` — similar to the role `d8:manage:all:viewer`, but with admin-level access, i.e., view/create/modify/delete the configuration of all DKP modules (`moduleConfig` resource), their cluster-wide resources, their namespaced resources, and standard Kubernetes objects in all system namespaces (starting with `d8-` or `kube-`);
  - `d8:manage:observability:viewer` — access to view the configuration of DKP modules (`moduleConfig` resource) from the `observability` area, their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except secrets and RBAC resources) in the system namespaces `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

The module provides two access level for administrators:
- `viewer` — allows viewing standard Kubernetes resources, the configuration of modules (resources `moduleConfig`), cluster-wide resources of modules, and namespaced resources of modules in the module namespace;
- `manager` — in addition to the role `viewer` it allows managing standard Kubernetes resources, the configuration of modules (resources `moduleConfig`), cluster-wide resources of modules, and namespaced resources of modules in the module namespace;

### Subsystems of the role-based model

Each DKP module belongs to a specific subsystem. For each subsystem, there is a set of roles with different levels of access. Roles are updated automatically when the module is enabled or disabled.

For example, for the `networking` subsystem, there are the following manage roles that can be used in `ClusterRoleBinding`: 
- `d8:manage:networking:viewer`
- `d8:manage:networking:manager`

The subsystem of the role restricts its action to all system namespaces of the cluster (subsystem `all`) or to those namespaces in which the area modules operate (see the table of area compositions).

Role-based model subsystems composition table.

{% include rbac/rbac-subsystems-list.liquid %}

<div style="height: 0;" id="the-obsolete-role-based-model"></div>

## Current role-based model

Features:
- Manages user and group access control using Kubernetes RBAC;
- Manages access to scaling tools (the `allowScale` parameter of the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-allowscale) or [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-allowscale) Custom Resource);
- Manages access to port forwarding (the `portForwarding` parameter of the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-portforwarding) or [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-portforwarding) Custom Resource);
- Manages the list of allowed namespaces with a labelSelector (the `namespaceSelector` parameter of the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector) Custom Resource);

In addition to the RBAC, you can use a set of high-level roles in the module:
- `User` — has access to information about all objects (including viewing pod logs) but cannot exec into containers, read secrets, and perform port-forwarding;
- `PrivilegedUser` — the same as `User` + can exec into containers, read secrets, and delete pods (and thus, restart them);
- `Editor` — is the same as `PrivilegedUser` + can create and edit all objects that are usually required for application tasks.
- `Admin` — the same as `Editor` + can delete service objects (auxiliary resources such as `ReplicaSet`, `certmanager.k8s.io/challenges` and `certmanager.k8s.io/orders`);
- `ClusterEditor` — the same as `Editor` + can manage a limited set of `cluster-wide` objects that can be used in application tasks (`ClusterXXXMetric`, `KeepalivedInstance`, `DaemonSet`, etc.). This role is best suited for cluster operators.
- `ClusterAdmin` — the same as both `ClusterEditor` and `Admin` + can manage `cluster-wide` service objects (e.g.,  `MachineSets`, `Machines`, `OpenstackInstanceClasses`..., as well as `ClusterAuthorizationRule`, `ClusterRoleBindings` and `ClusterRole`). This role is best suited for cluster administrators. **Note** that since `ClusterAdmin` can edit `ClusterRoleBindings`, he can **broaden his privileges within the cluster**;
- `SuperAdmin` — can perform any actions with any objects (note that `namespaceSelector` and `limitNamespaces` restrictions remain valid).

{% alert level="warning" %}
Currently, the multi-tenancy mode (namespace-based authorization) is implemented according to a temporary scheme and **isn't guaranteed to be entirely safe and secure**!
{% endalert %}

If a [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule) Custom Resource contains the `namespaceSelector` field, neither `limitNamespaces` nor `allowAccessToSystemNamespaces`are taken into consideration.

The `allowAccessToSystemNamespaces`, `namespaceSelector` and `limitNamespaces` options in the custom resource will no longer be applied if the authorization system's webhook is unavailable for some reason. As a result, users will have access to all namespaces. After the webhook availability is restored, the options will become relevant again.

### Default access list for each role

`verbs` aliases:
<!-- start user-authz roles placeholder -->
* read - `get`, `list`, `watch`
* read-write - `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`
* write - `create`, `delete`, `deletecollection`, `patch`, `update`

{{site.data.i18n.common.role[page.lang] | capitalize }} `User`:

```text
read:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - apps/deployments
    - apps/replicasets
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - configmaps
    - discovery.k8s.io/endpointslices
    - endpoints
    - events
    - events.k8s.io/events
    - extensions/daemonsets
    - extensions/deployments
    - extensions/ingresses
    - extensions/replicasets
    - extensions/replicationcontrollers
    - limitranges
    - metrics.k8s.io/nodes
    - metrics.k8s.io/pods
    - namespaces
    - networking.k8s.io/ingresses
    - networking.k8s.io/networkpolicies
    - nodes
    - persistentvolumeclaims
    - persistentvolumes
    - pods
    - pods/log
    - policy/poddisruptionbudgets
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - replicationcontrollers
    - resourcequotas
    - serviceaccounts
    - services
    - storage.k8s.io/storageclasses
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `PrivilegedUser` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`):

```text
create:
    - pods/eviction
create,get:
    - pods/attach
    - pods/exec
delete,deletecollection:
    - pods
read:
    - secrets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Editor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`):

```text
read-write:
    - apps/deployments
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - configmaps
    - discovery.k8s.io/endpointslices
    - endpoints
    - extensions/deployments
    - extensions/ingresses
    - networking.k8s.io/ingresses
    - persistentvolumeclaims
    - policy/poddisruptionbudgets
    - serviceaccounts
    - services
write:
    - secrets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Admin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create,patch,update:
    - pods
delete,deletecollection:
    - apps/replicasets
    - extensions/replicasets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterEditor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
read:
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
write:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - extensions/daemonsets
    - storage.k8s.io/storageclasses
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterAdmin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`):

```text
read-write:
    - deckhouse.io/clusterauthorizationrules
write:
    - limitranges
    - namespaces
    - networking.k8s.io/networkpolicies
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - resourcequotas
```
<!-- end user-authz roles placeholder -->

You can get additional list of access rules for module role from cluster ([existing user defined rules](usage.html#customizing-rights-of-high-level-roles) and non-default rules from other deckhouse modules):

```bash
D8_ROLE_NAME=Editor
kubectl get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```
