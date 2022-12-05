---
title: "The user-authz module" 
---

This module generates RBAC for users and implements the basic multi-tenancy mode with namespace-based access.

Also, it implements the role-based subsystem for end-to-end authorization, thereby extending the functionality of the standard RBAC mechanism.

All the configuration of access rights is performed using [Custom Resources](cr.html).

## Module features

- Manages user and group access control using Kubernetes RBAC;
- Manages access to scaling tools (the `allowScale` parameter of the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule) Custom Resource);
- Manages access to port forwarding (the `portForwarding` parameter of the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule) Custom Resource);
- Manages the list of allowed namespaces as regular expressions (the `limitNamespaces` parameter of the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule) Custom Resource);
- Manages access to system namespaces such as `kube-system`, etc., (the `allowAccessToSystemNamespaces` parameter of the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule) Custom Resource);

## Role model

In addition to the RBAC, you can use a set of high-level roles in the module:
- `User` — has access to information about all objects (including viewing Pod logs) but cannot exec into containers, read secrets, and perform port-forwarding;
- `PrivilegedUser` — the same as User + can exec into containers, read secrets, and delete Pods (and thus, restart them);
- `Editor` — the same as `PrivilegedUser` + can create and edit namespaces and all objects that are usually required for application tasks. **Note** that since `Editor` can edit `RoleBindings`, he can **broaden his privileges within the namespace**;
- `Admin` — the same as `Editor` + can delete service objects (auxiliary resources such as `ReplicaSet`, `certmanager.k8s.io/challenges` & `certmanager.k8s.io/orders`);
- `ClusterEditor` — the same as `Editor`+ can manage a limited set of `cluster-wide` objects that can be used in application tasks (`ClusterXXXMetric`, `ClusterRoleBindings`, `KeepalivedInstance`, `DaemonSet`, etc.). This role is best suited for cluster operators. **Note** that since `ClusterEditor` can edit `ClusterRoleBindings`, he can **broaden his privileges within the cluster**;
- `ClusterAdmin` — the same as both `ClusterEditor` and `Admin` + can manage `cluster-wide` service objects (e.g., `MachineSets`, `Machines`, `OpenstackInstanceClasses` and other resources). This role is best suited for cluster administrators. **Note** that since `ClusterAdmin` can edit `ClusterRoleBindings`, he can **broaden his privileges within the cluster**;
- `SuperAdmin` — can perform any actions with any objects (note that [`limitNamespaces`](#module-features) restrictions remain valid).

## Implementation nuances

**Caution!** Currently, the multi-tenancy mode (namespace-based authorization) is implemented according to a temporary scheme and **isn't guaranteed to be entirely safe and secure**!

The `allowAccessToSystemNamespaces` and `limitNamespaces` options in the CR will no longer be applied if the authorization system's webhook is unavailable for some reason. As a result, users will have access to all namespaces. After the webhook availability is restored, the options will become relevant again.
