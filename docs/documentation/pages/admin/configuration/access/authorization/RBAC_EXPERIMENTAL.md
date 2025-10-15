---
title: "Experimental authorization model"
permalink: en/admin/configuration/access/authorization/rbac-experimental.html
---

The experimental role model is designed on the principle of aggregation.
It combines low-level roles into bigger roles that cover common tasks.
This simplifies model extensibility by allowing you to add your own roles.

To use the experimental role model,
the [`user-authz`](/modules/user-authz/) module must be enabled in the cluster.
This module creates a set of cluster roles (ClusterRole) suitable for most user and group access management tasks.

{% alert level="warning" %}
Starting from Deckhouse Kubernetes Platform (DKP) v1.64, the module includes an experimental role-based access model.
The current role model will continue to function, but it will be deprecated in the future.

The current and experimental role-based access models are incompatible.
Automatic conversion of resources is not possible.
{% endalert %}

Unlike the [current role model](rbac-current.html) in DKP,
the experimental model does not use [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) or [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) resources.
Access control is configured using the standard Kubernetes RBAC approach
via [RoleBinding or ClusterRoleBinding](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#rolebinding-and-clusterrolebinding) resources referencing a role created by the `user-authz` module.

The module creates special aggregated cluster roles (ClusterRole).
By using these roles in [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) or [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/), you can achieve the following:

- Control access to modules associated with a specific [DKP subsystem](#role-model-subsystems).

  For example, to allow a user acting as a network administrator to configure *network* modules
  (such as [`cni-cilium`](/modules/cni-cilium/), [`ingress-nginx`](/modules/ingress-nginx/), [`istio`](/modules/istio/), etc.),
  you can use the `d8:manage:networking:manager` role in a ClusterRoleBinding.

- Control access to *user* module resources within a namespace.

  For example, assigning the `d8:use:role:manager` role via RoleBinding
  allows users to create, edit, or delete the [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) resource within a namespace.
  However, it does not grant access to cluster-wide resources such as [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) or [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination)
  in the `log-shipper` module, nor does it allow configuring the `log-shipper` module itself.

The roles created by the module fall into two categories:

- [Use roles](#use-roles) — for granting permissions to users (for example, application developers) *within a specific namespace*.
- [Manage roles](#manage-roles) — for granting administrator permissions.

## Use roles

{% alert level="warning" %}
Use rules can only be used in [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) resources.
{% endalert %}

Use roles are intended for granting permissions to users *within a specific namespace*.
"Users" in this context typically refers to developers deploying applications in a cluster configured by an administrator.
These users don’t need to manage DKP modules or the cluster,
but they do need permissions to create Ingress resources, configure application authentication, collect logs, etc.

Use roles define access to namespaced module resources and standard Kubernetes namespaced resources
such as Pod, Deployment, Secret, ConfigMap, etc.

The [`user-authz`](/modules/user-authz/) module creates the following use roles:

| Role | Allowed actions | Access restrictions |
|------|-----------------|---------------------|
| `d8:use:role:viewer` | View Pod, Deployment, Service (except for secrets and RBAC) | Can't access `exec`, ports or resource changes |
| `d8:use:role:user` | Read secrets, `kubectl exec`, `port-forward`, delete Pods, scale replicas | Can't create or edit objects |
| `d8:use:role:manager` | Create or modify Pod, ConfigMap, Deployment, manage module resources (such as Certificate) | Can't access Quota or RBAC |
| `d8:use:role:admin` | Manage ServiceAccount, Role, ResourceQuota, NetworkPolicy | Full access within a namespace |

Key role differences:

- `viewer`: `read-only` (no access to secrets or RBAC).
- `user`: Adds access to secrets, Pods, and networking features (port-forward).
- `manager`: Allows managing application resources and related module resources.
- `admin`: Full control over a namespace (including RBAC and quotas).

## Manage roles

{% alert level="warning" %}
Manage roles do not grant access to namespaces used by user applications.

A manage role only grants access to system namespaces (those starting with `d8-` or `kube-`),
and only to those where the modules associated with the role’s subsystem are running.
{% endalert %}

Manage roles are intended for granting permissions to administer the entire DKP or its parts ([specific subsystems](#role-model-subsystems)),
but not for managing user applications.
For example, you can use a manage role to allow a security administrator to manage modules
responsible for the cluster’s security functions.
Such an administrator will be able to manage authentication, authorization, and security policies,
but will not have access to other areas of the cluster (such as networking or monitoring subsystems)
and will not be able to change anything in user application namespaces.

A manage role grants access to:

- Kubernetes cluster-wide resources.
- Management of DKP modules (ModuleConfig resources) within the role’s [subsystem](#role-model-subsystems)
  or across all DKP modules if using the `d8:manage:all:*` role.
- Management of DKP module cluster-wide resources within the role’s [subsystem](#role-model-subsystems)
  or all DKP module resources if using the `d8:manage:all:*` role.
- System namespaces (starting with `d8-` or `kube-`) in which the modules from the role’s [subsystem](#role-model-subsystems) are running
  or all system namespaces in the case of the `d8:manage:all:*` role.

The naming format for a manage role is `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>`, where:

- `SUBSYSTEM` is the role’s subsystem. It can either be one of the [listed subsystems](#role-model-subsystems), or `all` to cover all subsystems.
- `ACCESS_LEVEL` is the access level.

Examples of manage roles:

- `d8:manage:all:viewer`: View access to the configuration of all DKP modules (ModuleConfig resources),
  their cluster-wide and namespaced resources, and standard Kubernetes objects (excluding secrets and RBAC resources)
  in all system namespaces (starting with `d8-` or `kube-`).
- `d8:manage:all:manager`: Same as `d8:manage:all:viewer`, but with `admin`-level access,
  meaning, permission to view, create,edit, and delete configurations of all DKP modules (ModuleConfig resources),
  their cluster-wide and namespaced resources, and standard Kubernetes objects in all system namespaces
  (starting with `d8-` or `kube-`).
- `d8:manage:observability:viewer`: View access to DKP module configurations (ModuleConfig resources)
  in the `observability` subsystem, their cluster-wide and namespaced resources, and standard Kubernetes objects
  (excluding secrets and RBAC resources) in system namespaces such as `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`,
  `d8-operator-prometheus`, `d8-upmeter`, and `kube-prometheus-pushgateway`.

The module provides two access levels for administrators:

- `viewer`: Allows viewing standard Kubernetes resources, module configurations (ModuleConfig resources),
  module cluster-wide resources, and namespaced module resources within the module’s namespace.
- `manager`: In addition to `viewer` permissions, allows managing standard Kubernetes resources,
  module configurations (ModuleConfig resources), module cluster-wide resources,
  and namespaced module resources within the module’s namespace.

## Role model subsystems

Each DKP module belongs to a specific subsystem.
For each subsystem, there is a set of roles with different access levels.
These roles are automatically updated when a module is enabled or disabled.

For example, the `networking` subsystem includes the following manage roles, which can be used in [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/):

- `d8:manage:networking:viewer`
- `d8:manage:networking:manager`

A role’s subsystem limits its scope to either all system namespaces (those starting with `d8-` or `kube-`) if using `all`,
or only those namespaces where modules of the specified subsystem are running (refer to the subsystem composition table for details).

### Role model subsystem composition

{% include rbac/rbac-subsystems-list.liquid %}

## Creating a new subsystem role

If the current subsystems don't meet requirements of the role distribution used in the company,
you can create a new [subsystem](#role-model-subsystems) that will include roles from the `deckhouse` and `kubernetes` subsystems
and the [`user-authn`](/modules/user-authn/) module.

To create a role, use the following template:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: subsystem
    rbac.deckhouse.io/subsystem: custom
    rbac.deckhouse.io/aggregate-to-all-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        module: user-authn
rules: []
```

### Label descriptions

A created role includes the following labels:

- `rbac.deckhouse.io/use-role: admin`: Indicates which role the hook should use when creating use roles.
- `rbac.deckhouse.io/kind: manage`: Specifies that the role's type is `manage`. **This label is required**.
- `rbac.deckhouse.io/level: subsystem`: Indicates that the role belongs to a subsystem level and will be processed accordingly.
- `rbac.deckhouse.io/subsystem: custom`: Defines the name of the subsystem this role is responsible for.
- `rbac.deckhouse.io/aggregate-to-all-as: manager`: Allows the `manage:all` role to aggregate this role as a `manager`.

### Aggregation selector descriptions

The `aggregationRule` section defines which roles and modules are aggregated into this role:

- `rbac.deckhouse.io/kind: manage`, `rbac.deckhouse.io/aggregate-to-deckhouse-as: manager`:
  Aggregates a manage role from the `deckhouse` and `kubernetes` subsystems.
- `rbac.deckhouse.io/kind: manage`, `module: user-authn`: Aggregates all rules from the `user-authn` module.

This way, the role inherits permissions from the `deckhouse` and `kubernetes` subsystems as well as from the [`user-authn`](/modules/user-authn/) module.

{% alert level="info" %}

- There are no restrictions on the role name, but it is recommended that you follow a readable and consistent naming style.
- Use roles will be automatically created in the namespaces of the corresponding subsystems and modules.
  The role type is determined by its label.

{% endalert %}

## Extending a custom role

In the following example, a new cluster-wide CRD object called MySuperResource has been introduced into the cluster.
To grant access to it, you need to extend an existing manage role described above.

1. Add a new aggregation selector to the role:

   ```yaml
   rbac.deckhouse.io/kind: manage
   rbac.deckhouse.io/aggregate-to-custom-as: manager
   ```

   This selector allows including the role in a subsystem’s aggregating role via the corresponding label.
   After adding the new selector, the role will look as follows:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: custom:manage:mycustom:manager
     labels:
       rbac.deckhouse.io/use-role: admin
       rbac.deckhouse.io/kind: manage
       rbac.deckhouse.io/level: subsystem
       rbac.deckhouse.io/subsystem: custom
       rbac.deckhouse.io/aggregate-to-all-as: manager
   aggregationRule:
     clusterRoleSelectors:
       - matchLabels:
           rbac.deckhouse.io/kind: manage
           rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
       - matchLabels:
           rbac.deckhouse.io/kind: manage
           rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
       - matchLabels:
           rbac.deckhouse.io/kind: manage
           module: user-authn
       - matchLabels:
           rbac.deckhouse.io/kind: manage
           rbac.deckhouse.io/aggregate-to-custom-as: manager
   rules: []
   ```

1. Create a new role to define permissions for the new resource.
   Example configuration for the read-only permissions:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     labels:
       rbac.deckhouse.io/aggregate-to-custom-as: manager
       rbac.deckhouse.io/kind: manage
     name: custom:manage:permission:mycustom:superresource:view
   rules:
   - apiGroups:
     - mygroup.io
     resources:
     - mysuperresources
     verbs:
     - get
     - list
     - watch
   ```

   The new role's permissions will be added to the subsystem role, providing view access to the new object.

{% alert level="info" %}
There are no restrictions on the role name, but it is recommended that you follow the consistent naming style.
{% endalert %}

## Extending existing manage-subsystem roles

To extend an existing role, follow the same steps as above but make sure to change the role name and labels.

Example configuration for extending the manage role from the `deckhouse` subsystem (`d8:manage:deckhouse:manager`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: manage
  name: custom:manage:permission:mycustommodule:superresource:view
rules:
- apiGroups:
  - mygroup.io
  resources:
  - mysuperresources
  verbs:
  - get
  - list
  - watch
```

This way the new role will extend the `d8:manage:deckhouse` role.

## Extending manage-subsystem roles and adding a namespace

To add a new namespace for creating a use role in it via the hook, you would need only one label:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

This label informs the hook that a use role needs to be created in this namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/namespace: namespace
  name: custom:manage:permission:mycustom:superresource:view
rules:
- apiGroups:
  - mygroup.io
  resources:
  - mysuperresources
  verbs:
  - get
  - list
  - watch
```

The hook watches [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) resources and, upon their creation, inspects the associated manage roles
to find all aggregated roles defined via the `aggregationRule`.
For each aggregated role, it extracts the namespace from the `rbac.deckhouse.io/namespace` label
and creates a corresponding use role in that namespace.

## Extending existing use roles

If a resource is part of a namespace, you will need to extend the use role instead of the manage role.
The only difference is the name and label:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-kubernetes-as: user
    rbac.deckhouse.io/kind: use
  name: custom:use:capability:mycustom:superresource:view
rules:
- apiGroups:
  - mygroup.io
  resources:
  - mysuperresources
  verbs:
  - get
  - list
  - watch
```

This role extends the `d8:use:role:user:kubernetes` role.
