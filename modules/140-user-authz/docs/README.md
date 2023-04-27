---
title: "The user-authz module"
---

The module generates RBAC for users and implements the basic multi-tenancy mode with namespace-based access.

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
- `User` — has access to information about all objects (including viewing pod logs) but cannot exec into containers, read secrets, and perform port-forwarding;
- `PrivilegedUser` — the same as `User` + can exec into containers, read secrets, and delete pods (and thus, restart them);
- `Editor` — is the same as `PrivilegedUser` + can create and edit all objects that are usually required for application tasks.
- `Admin` — the same as `Editor` + can delete service objects (auxiliary resources such as `ReplicaSet`, `certmanager.k8s.io/challenges` and `certmanager.k8s.io/orders`), also allows you to control access to namespace resources via `RoleBindings` and `Role`. **Note** that since `Admin` can edit `RoleBindings`, he can **broaden his privileges within the namespace**;
- `ClusterEditor` — the same as `Editor` + can manage a limited set of `cluster-wide` objects that can be used in application tasks (`ClusterXXXMetric`, `ClusterRoleBindings`, `KeepalivedInstance`, `DaemonSet`, etc.). This role is best suited for cluster operators.
- `ClusterAdmin` — the same as both `ClusterEditor` and `Admin` + can manage `cluster-wide` service objects (e.g.,  `MachineSets`, `Machines`, `OpenstackInstanceClasses`..., as well as `ClusterAuthorizationRule`, `ClusterRoleBindings` и `ClusterRole`). This role is best suited for cluster administrators. **Note** that since `ClusterAdmin` can edit `ClusterRoleBindings`, he can **broaden his privileges within the cluster**;
- `SuperAdmin` — can perform any actions with any objects (note that [`limitNamespaces`](#module-features) restrictions remain valid).

## Implementation nuances

> **Caution!** Currently, the multi-tenancy mode (namespace-based authorization) is implemented according to a temporary scheme and **isn't guaranteed to be entirely safe and secure**!

The `allowAccessToSystemNamespaces` and `limitNamespaces` options in the CR will no longer be applied if the authorization system's webhook is unavailable for some reason. As a result, users will have access to all namespaces. After the webhook availability is restored, the options will become relevant again.

## Default access list for each role:
<!-- start placeholder -->
`verbs` aliases:
*read - `get`, `list`, `watch`
*read-write - `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`
*write - `create`, `delete`, `deletecollection`, `patch`, `update`

Role `User`:
  read:
  - events.k8s.io/events
  - metrics.k8s.io/nodes
  - persistentvolumeclaims
  - services
  - extensions/ingresses
  - endpoints
  - rbac.authorization.k8s.io/rolebindings
  - events
  - nodes
  - policy/poddisruptionbudgets
  - metrics.k8s.io/pods
  - networking.k8s.io/networkpolicies
  - replicationcontrollers
  - resourcequotas
  - limitranges
  - batch/jobs
  - serviceaccounts
  - apps/replicasets
  - extensions/replicationcontrollers
  - apps/daemonsets
  - discovery.k8s.io/endpointslices
  - apiextensions.k8s.io/customresourcedefinitions
  - apps/statefulsets
  - namespaces
  - pods
  - autoscaling/horizontalpodautoscalers
  - persistentvolumes
  - rbac.authorization.k8s.io/roles
  - storage.k8s.io/storageclasses
  - batch/cronjobs
  - apps/deployments
  - pods/log
  - extensions/replicasets
  - extensions/daemonsets
  - extensions/deployments
  - autoscaling.k8s.io/verticalpodautoscalers
  - configmaps
  - networking.k8s.io/ingresses
  read-write:
  - events.k8s.io/events
  - metrics.k8s.io/nodes
  - persistentvolumeclaims
  - services
  - extensions/ingresses
  - endpoints
  - rbac.authorization.k8s.io/rolebindings
  - events
  - nodes
  - policy/poddisruptionbudgets
  - metrics.k8s.io/pods
  - networking.k8s.io/networkpolicies
  - replicationcontrollers
  - resourcequotas
  - limitranges
  - batch/jobs
  - serviceaccounts
  - apps/replicasets
  - extensions/replicationcontrollers
  - apps/daemonsets
  - discovery.k8s.io/endpointslices
  - apiextensions.k8s.io/customresourcedefinitions
  - apps/statefulsets
  - namespaces
  - pods
  - autoscaling/horizontalpodautoscalers
  - persistentvolumes
  - rbac.authorization.k8s.io/roles
  - storage.k8s.io/storageclasses
  - batch/cronjobs
  - apps/deployments
  - pods/log
  - extensions/replicasets
  - extensions/daemonsets
  - extensions/deployments
  - autoscaling.k8s.io/verticalpodautoscalers
  - configmaps
  - networking.k8s.io/ingresses

Role `PrivilegedUser` (and all rules from `User`):
  read:
  - pods/exec
  - pods/attach
  - secrets
  read-write:
  - pods/exec
  - pods/attach
  - secrets
  write:
  - pods
  - pods/exec
  - pods/attach

Role `Editor` (and all rules from `User`, `PrivilegedUser`):
  write:
  - discovery.k8s.io/endpointslices
  - apps/statefulsets
  - persistentvolumeclaims
  - services
  - extensions/ingresses
  - endpoints
  - autoscaling/horizontalpodautoscalers
  - batch/cronjobs
  - policy/poddisruptionbudgets
  - secrets
  - apps/deployments
  - batch/jobs
  - serviceaccounts
  - extensions/deployments
  - autoscaling.k8s.io/verticalpodautoscalers
  - configmaps
  - networking.k8s.io/ingresses

Role `Admin` (and all rules from `User`, `PrivilegedUser`, `Editor`):
  write:
  - rbac.authorization.k8s.io/roles
  - apps/replicasets
  - extensions/replicasets
  - rbac.authorization.k8s.io/rolebindings

Role `ClusterEditor` (and all rules from `User`, `PrivilegedUser`, `Editor`):
  read:
  - rbac.authorization.k8s.io/clusterroles
  - rbac.authorization.k8s.io/clusterrolebindings
  read-write:
  - rbac.authorization.k8s.io/clusterroles
  - rbac.authorization.k8s.io/clusterrolebindings
  write:
  - storage.k8s.io/storageclasses
  - apiextensions.k8s.io/customresourcedefinitions
  - extensions/daemonsets
  - apps/daemonsets

Role `ClusterAdmin` (and all rules from `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`):
  read:
  - deckhouse.io/clusterauthorizationrules
  read-write:
  - deckhouse.io/clusterauthorizationrules
  write:
  - networking.k8s.io/networkpolicies
  - resourcequotas
  - limitranges
  - namespaces
  - rbac.authorization.k8s.io/clusterroles
  - deckhouse.io/clusterauthorizationrules
  - rbac.authorization.k8s.io/clusterrolebindings

<!-- end placeholder -->

You can get additional list of access rules for module role from cluster ([existing user defined rules](usage.html#customizing-rights-of-high-level-roles) and non-default rules from other deckhouse modules):
```bash
D8_ROLE_NAME=Editor
kubectl get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```
