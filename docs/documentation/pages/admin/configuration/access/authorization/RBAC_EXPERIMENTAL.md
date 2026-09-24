---
title: "Granular authorization model"
permalink: en/admin/configuration/access/authorization/rbac-experimental.html
description: "Configure the granular (scope-based) RBAC authorization model in Deckhouse Platform: role scopes, access levels, project roles, and the user-authz module setup."
---

The granular role model is built on the principle of aggregation: it combines small, single-purpose roles (capabilities) into bigger roles that cover common tasks. This makes the model easy to extend by adding your own roles.

To use the granular role model, the [`user-authz`](/modules/user-authz/) module must be enabled in the cluster. This module creates a set of cluster roles (ClusterRole) suitable for most user and group access management tasks.

{% alert level="warning" %}
The module implements two role-based models: the granular one (this page, recommended) and the [basic](rbac-current.html) one, built around the ClusterAuthorizationRule and AuthorizationRule resources (its support will be discontinued in future releases).

The models are not resource-compatible — automatic conversion is impossible — but they can be used at the same time: the permissions of both models are summed up.
{% endalert %}

Unlike the [basic role model](rbac-current.html), the granular one does not use [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) or [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) resources. Access rights are configured the standard Kubernetes RBAC way: by creating [RoleBinding or ClusterRoleBinding](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#rolebinding-and-clusterrolebinding) resources and specifying one of the roles prepared by the `user-authz` module in them. To grant access to all namespaces of a project at once, use the [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) and [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) resources of the `multitenancy-manager` module.

{% alert level="info" %}
Access does not have to be granted by hand-writing YAML manifests: the [Deckhouse Kubernetes Platform web interface](/products/kubernetes-platform/documentation/latest/user/web/ui.html) provides an access grant wizard. It walks you through the steps (who gets access → where → at which level), picks the right binding kind itself (RoleBinding, ClusterRoleBinding, ProjectRoleBinding, or ClusterProjectRoleBinding), and lets you assemble a custom role from ready-made building blocks without writing YAML.
{% endalert %}

The module creates special aggregated cluster roles (ClusterRole). By using these roles in RoleBinding or ClusterRoleBinding, you can do the following:

- Manage access to modules of a specific [subsystem](#role-model-subsystems).

  For example, you can use the `d8:subsystem:networking:manager` role in a ClusterRoleBinding to allow a network administrator to configure *network* modules (such as [`cni-cilium`](/modules/cni-cilium/), [`ingress-nginx`](/modules/ingress-nginx/), [`istio`](/modules/istio/), etc.).
- Manage access to *user* resources of modules within namespaces.

  For example, the `d8:namespace:manager` role in a RoleBinding enables deleting/creating/editing the [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) resource in the namespace. At the same time, it does not grant access to the cluster-wide [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) and [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) resources of the `log-shipper` module, nor does it allow configuration of the `log-shipper` module itself.

The roles created by the module are divided into the following classes:

- [Namespace roles](#namespace-roles) — for assigning rights to users (such as application developers) **in a specific namespace**.
- [Project roles](#project-roles) — for assigning rights **in all namespaces of a project at once**.
- [System and subsystem roles](#system-and-subsystem-roles) — for assigning rights to platform administrators and administrators of a part of the platform.

## Role scopes

Every role operates in one of four scopes. The scope defines *where* the granted permissions apply and *which resource* is used to assign the role:

| Scope | Role name format | Intended for | Which resource is used to assign the role |
|-------|------------------|--------------|---------------------------------------------|
| Namespace | `d8:namespace:<level>` | Application users (developers) | RoleBinding in a specific namespace. |
| Project | `d8:project:<level>` | Teams working with [projects](/modules/multitenancy-manager/) | Only [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) or [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). |
| Subsystem | `d8:subsystem:<subsystem>:<level>` | Administrators of a part of the platform | ClusterRoleBinding. |
| Whole platform | `d8:system:<level>` | Platform administrators | ClusterRoleBinding. |

### Access levels

Each scope provides several access levels for roles:

- The "namespace" and "project" scopes have five levels: `viewer` → `user` → `manager` → `admin` → `superadmin`.
- The "subsystem" and "whole platform" scopes have three levels: `viewer` → `manager` → `superadmin`. There are no `user` and `admin` levels here: the system level has no "user" resources that could be used without administering them.

Access levels form a hierarchy with a cumulative principle: each next level inherits all permissions of the previous one and extends them. For example, the `manager` level in the namespace scope includes everything the `user` level allows, which in turn includes everything the `viewer` level allows.

## Namespace roles

{% alert level="warning" %}
A namespace role can only be used in a RoleBinding resource.
{% endalert %}

Namespace roles are intended to assign rights to a user **in a specific namespace**. Users refer to, for example, developers who use a cluster configured by an administrator to deploy their applications. Such users don't need to manage DP modules or a cluster, but they need to be able to, for example, create their Ingress resources, configure application authentication, and collect logs from applications.

A namespace role defines permissions for accessing namespaced resources of modules and standard namespaced resources of Kubernetes (Pod, Deployment, Secret, ConfigMap, etc.).

The module creates the following namespace roles:

| Role | Allowed actions | Access restrictions |
|------|------------------|----------------------|
| `d8:namespace:viewer` | View standard Kubernetes resources (except Secrets and RBAC resources), pod logs and metrics, and authenticate in the cluster | No access to secrets, `exec`, or resource changes |
| `d8:namespace:user` | Everything `viewer` allows, plus: view secrets, `kubectl exec`/`attach`, delete pods (but not create or modify them), `kubectl port-forward`/`proxy`, change controller replica counts | Can't create or edit objects |
| `d8:namespace:manager` | Everything `user` allows, plus: manage module resources (for example, Certificate, PodLoggingConfig) and standard namespaced Kubernetes resources (Pod, Deployment, ConfigMap, Secret, Service, Ingress, NetworkPolicy, CronJob, etc.) | No access to quotas or RBAC |
| `d8:namespace:admin` | Everything `manager` allows, plus: manage ResourceQuota, LimitRange, ServiceAccount, Role, RoleBinding | Full access within a namespace, except the operations reserved for `superadmin` |
| `d8:namespace:superadmin` | Everything `admin` allows, plus security-sensitive operations: minting ServiceAccount tokens, making requests on behalf of ServiceAccounts, and managing [system resources placed in the namespace](#admin-level-restrictions-and-superadmin-rights) (for example, Dex pods or pods/PVCs of virtual machines) | — |

The detailed split of permissions between `admin` and `superadmin` is described in [Admin level restrictions and superadmin rights](#admin-level-restrictions-and-superadmin-rights).

### Automatic access to cluster-wide catalog resources

Working in a namespace requires reading some cluster-wide "catalogs": for example, to set `storageClassName` or `ingressClassName` in a manifest, one needs to see the list of StorageClasses and IngressClasses. Therefore, every subject that receives a RoleBinding to any `d8:namespace:*` role automatically also gets read access to such catalog resources (StorageClass, IngressClass, PriorityClass, RuntimeClass, VolumeSnapshotClass, ClusterLogDestination, etc.).

Technically, this looks like an automatically created ClusterRoleBinding to the [`d8:dict`](#global-resource-dictionaries) role with the `rbac.deckhouse.io/dict: "true"` label. Such objects are managed by the platform: they appear when the subject is granted their first namespace binding and are removed once none are left — there is no need to edit them manually.

## Project roles

{% alert level="warning" %}
A project role cannot be assigned via ClusterRoleBinding — such an attempt is rejected. To assign a role across a whole project, use [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) or [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). A plain RoleBinding in one of the project namespaces is also allowed — the role then applies in that namespace only.
{% endalert %}

Project roles (`d8:project:<level>`) are intended for working with [projects](/modules/multitenancy-manager/) — isolated environments that may span several namespaces. The levels are the same as for namespace roles: `viewer`, `user`, `manager`, `admin`, `superadmin`.

Each project role includes all permissions of the namespace role of the same level and additionally grants permissions to manage the project itself:

- `d8:project:viewer` — the permissions of `d8:namespace:viewer` plus viewing the project's ProjectNamespace and ProjectRoleBinding resources;
- `d8:project:manager` — the permissions of `d8:namespace:manager` plus managing the project's additional namespaces (ProjectNamespace resources);
- `d8:project:admin` — the permissions of `d8:namespace:admin` plus managing access to the project (ProjectRoleBinding resources) and the right to bind the built-in `d8:project:*` and `d8:namespace:*` roles (except the `superadmin` level) to other users within the project;
- `d8:project:superadmin` — analogous to the relation between `d8:namespace:superadmin` and `d8:namespace:admin`.

A role assigned via ProjectRoleBinding automatically applies in **all** namespaces of the project — the main one as well as the additional ones, including those created later.

## System and subsystem roles

{% alert level="warning" %}
System and subsystem roles do not grant access to the namespaces of user applications.

They grant access only to system namespaces (starting with `d8-` or `kube-`), and only to those system namespaces where the modules of the corresponding role subsystem are running.

A system or subsystem role does not, by itself, let you grant access to other people: creating a User or Group for an email that already carries a grant, or writing a ClusterAuthorizationRule, is *granting* roles, and the request is admitted only if the requester already has covering permissions or is explicitly allowed to assign those roles.
{% endalert %}

System and subsystem roles are intended for assigning rights to manage the entire platform or a part of it (the [subsystem](#role-model-subsystems)), but not the user applications themselves. The subsystem role, for example, can allow a security administrator to manage security modules (responsible for the security functions of the cluster). Thus, the security administrator will be able to configure authentication, authorization, security policies, etc., but will not be able to manage other cluster functions (such as network and monitoring settings) or change settings in the namespaces of user applications.

{% alert level="warning" %}
A system/subsystem role limits which modules and namespaces a subject can address, but it does not limit the privileges a subject can obtain through the modules it is allowed to manage. This matters most for the `security` subsystem: the right to manage authentication and authorization is equivalent to full control over the cluster — a subject that can manage the `user-authn` module can register an identity provider or reset the credentials of any local user, and a subject that can manage the `user-authz` module can modify authorization rules. In both cases, they can obtain an identity with any privileges, including cluster administrator, so treat a `security` subsystem role as a cluster administrator role when planning access.
{% endalert %}

The system/subsystem role defines access rights:

- to cluster-wide Kubernetes resources;
- to manage DP modules (ModuleConfig resource) within the [subsystem](#role-model-subsystems) of the role, or to all DP modules for the role `d8:system:*`;
- to manage cluster-wide resources of DP modules within the [subsystem](#role-model-subsystems) of the role, or to all resources of DP modules for the role `d8:system:*`;
- to system namespaces (starting with `d8-` or `kube-`) in which the modules of the [subsystem](#role-model-subsystems) of the role operate, or to all system namespaces for the role `d8:system:*`.

The role name format is `d8:system:<ACCESS_LEVEL>` for the system roles and `d8:subsystem:<SUBSYSTEM>:<ACCESS_LEVEL>` for the subsystem roles, where:

- `SUBSYSTEM` is the role's [subsystem](#role-model-subsystems);
- `ACCESS_LEVEL` is the access level.

Examples:

- `d8:system:viewer` — access to view the configuration of all DP modules (ModuleConfig resource), their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except Secrets and RBAC resources) in all system namespaces (starting with `d8-` or `kube-`);
- `d8:system:manager` — similar to `d8:system:viewer`, but with admin-level access, i.e., view/create/modify/delete the configuration of all DP modules (ModuleConfig resource), their cluster-wide resources, their namespaced resources, and standard Kubernetes objects in all system namespaces;
- `d8:subsystem:observability:viewer` — access to view the configuration of DP modules (ModuleConfig resource) from the `observability` subsystem, their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except Secrets and RBAC resources) in the system namespaces `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

The module provides three access levels for system and subsystem roles:

- `viewer` — allows viewing standard Kubernetes resources, the configuration of modules (ModuleConfig resources), cluster-wide resources of modules, and namespaced resources of modules in the module namespace;
- `manager` — in addition to `viewer`, allows managing standard Kubernetes resources, the configuration of modules (ModuleConfig resources), cluster-wide resources of modules, and namespaced resources of modules in the module namespace;
- `superadmin` — in addition to `manager`, allows managing system resources of the subsystem modules.

## Global resource dictionaries

In addition to the roles and capabilities of the granular model, the module creates a special ClusterRole `d8:dict`. It grants **read-only** access to cluster-scoped "reference" resources that users commonly need to discover when creating objects — for example, a user creating a PersistentVolumeClaim needs to see the available StorageClasses, and a user creating an Ingress needs to see the IngressClasses.

The role grants `get`, `list`, `watch` on the following resources:

- `storageclasses` (storage.k8s.io), plus `csidrivers`, `csinodes`, `volumeattachments`;
- `volumesnapshotclasses` (snapshot.storage.k8s.io);
- `ingressclasses` (networking.k8s.io);
- `priorityclasses` (scheduling.k8s.io);
- `runtimeclasses` (node.k8s.io);
- `virtualmachineclasses`, `clustervirtualimages` (virtualization.deckhouse.io);
- `clusterlogdestinations` (deckhouse.io);
- `customresourcedefinitions` (apiextensions.k8s.io) — `get`, `list` only.

### Automatic binding

The `d8:dict` ClusterRoleBinding is created **automatically** whenever a RoleBinding references a `d8:namespace:*` or `d8:project:*` role (granular model) or a `user-authz:*` role (basic model). The RoleBindings that `multitenancy-manager` fans out from a ProjectRoleBinding or a ClusterProjectRoleBinding count as well, so the holders of project roles get the binding too. This means project users can discover reference resources without any manual configuration. The binding appears with the RoleBinding and is removed when it is deleted. The binding is created and managed entirely by the `user-authz-controller` component of the module. You do not create it yourself.

{% alert level="info" %}
The `d8:dict` role is independent of the `multitenancy-manager` cluster-wide resource access management mechanism: it only gives **read** access to reference resources so users can discover them. The mechanism controls *which resource values* a project may actually reference when creating objects.
{% endalert %}

## Role model subsystems

Each DP module belongs to a specific subsystem. For each subsystem, there is a set of roles with different levels of access. Roles are updated automatically when the module is enabled or disabled.

For example, for the `networking` subsystem, there are the following subsystem roles that can be used in [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/):

- `d8:subsystem:networking:viewer`;
- `d8:subsystem:networking:manager`;
- `d8:subsystem:networking:superadmin`.

The scope of a role depends on which subsystem it belongs to:

- The scope of the `d8:system:*` roles is all system namespaces (starting with `d8-` or `kube-`) in the cluster.
- The scope of subsystem roles includes the namespaces in which the subsystem's modules operate (see the subsystem composition table below), as well as all cluster-wide objects of the subsystem's modules.

Role model subsystems composition table.

{% include rbac/rbac-subsystems-list.liquid %}

## How the roles are built: aggregation and capabilities

No built-in role contains a list of permissions directly. Permissions are described in separate small cluster roles — **capabilities**. Each capability is responsible for one kind of action (for example, "view logs", "manage quotas", "connect to pods") and contains concrete RBAC rules. A role (`d8:namespace:admin`, `d8:system:viewer`, etc.) is an empty ClusterRole with an aggregation rule (`aggregationRule`): Kubernetes automatically collects into it the rules from all capabilities with matching labels.

This design has two practical consequences:

- DP modules extend the roles automatically: when a module is enabled, its capabilities are added to the corresponding built-in roles; when it is disabled, they are removed. The permission list of a role always matches the set of enabled modules.
- You can assemble your own roles from ready-made capabilities without writing RBAC rules by hand, as shown below.

The names of built-in roles and capabilities start with the `d8:` prefix. This namespace is reserved: you cannot create your own ClusterRole with a `d8:*` name — the only exception is the `d8:custom:*` prefix, which is dedicated to user-defined roles and capabilities.

## Creating a new subsystem role

Suppose that the current [subsystems](#role-model-subsystems) do not fit the role distribution in the company. You need to create a new subsystem that includes roles from the `deckhouse` subsystem, the `kubernetes` subsystem and the `user-authn` module.

To meet this need, create the following role:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
    rbac.deckhouse.io/aggregate-to-system-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    - matchLabels:
        rbac.deckhouse.io/scope: system
        module: user-authn
rules: []
```

The labels for the new role listed at the top suggest that:

- The hook will use this namespace role when creating a RoleBinding in the module namespaces:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- The role is a custom role (custom roles never define their own rules, they only aggregate capabilities):

  ```yaml
  rbac.deckhouse.io/kind: custom-role
  ```

  > Note that this label is mandatory.

- The role is a subsystem one, and it shall be handled accordingly:

  ```yaml
  rbac.deckhouse.io/scope: subsystem
  ```

- There is a subsystem for which the role is responsible:

  ```yaml
  rbac.deckhouse.io/subsystem: mycustom
  ```

- The `d8:system:manager` role can aggregate this role:

  ```yaml
  rbac.deckhouse.io/aggregate-to-system-as: manager
  ```

Then there are selectors that implement aggregation:

- This one aggregates the manager role from the `deckhouse` subsystem:

  ```yaml
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- This one aggregates all the system-scope capabilities defined for the `user-authn` module:

  ```yaml
   rbac.deckhouse.io/scope: system
   module: user-authn
  ```

{% alert level="info" %}
For more information on role labels and annotations, see [Reference of role labels and annotations](/modules/user-authz/#reference-of-role-labels-and-annotations).
{% endalert %}

This way, your role will combine the permissions of the `deckhouse` subsystem, `kubernetes` subsystem, and the `user-authn` module.

Notes:

- Custom roles and capabilities must be named with the `d8:custom:` prefix (the rest of the `d8:` prefix space is reserved for DP built-in objects). The name must agree with the declared scope: a subsystem role is `d8:custom:<subsystem>:<name>` (the segment is the subsystem itself, as in the example above), a namespace or project role is `d8:custom:namespace:<name>` or `d8:custom:project:<name>`, and a capability is `d8:custom:<scope>-capability:<name>`. A name that disagrees with the `rbac.deckhouse.io/scope` label is rejected.
- RoleBindings with a namespace role (`d8:namespace:<level>`) will be created in the namespaces of the aggregated subsystems' modules; the level is specified by the `rbac.deckhouse.io/use-role` label.

## Extending a custom role

Suppose a new cluster CRD object, MySuperResource, has been created in the cluster (a manage role example), and you need to extend the custom role from the example above to include the permissions to interact with this resource.

First, you have to add a new selector to the role:

```yaml
rbac.deckhouse.io/aggregate-to-mycustom-as: manager
```

For more information on role labels and annotations, see [Reference of role labels and annotations](/modules/user-authz/#reference-of-role-labels-and-annotations).

This selector would enable capabilities to be aggregated to a new subsystem by specifying this label. After adding the new selector, the role will look as follows:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   name: d8:custom:mycustom:manager
   labels:
     rbac.deckhouse.io/use-role: admin
     rbac.deckhouse.io/kind: custom-role
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: mycustom
     rbac.deckhouse.io/aggregate-to-system-as: manager
 aggregationRule:
   clusterRoleSelectors:
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
     - matchLabels:
         rbac.deckhouse.io/scope: system
         module: user-authn
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-mycustom-as: manager
 rules: []
 ```

 Next, you need to create a new capability and define permissions for the new resource, e.g., the read-only permission:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-mycustom-as: manager
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: mycustom
     rbac.deckhouse.io/capability: "custom.subsystem-capability.mycustom.superresource_view"
   name: d8:custom:subsystem-capability:mycustom:superresource:view
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

The capability will add its permissions to the subsystem role, so that holders of the role will be able to view the new object.

Notes:

- Custom capabilities must be named with the `d8:custom:` prefix; the rest of the name is not restricted, but we recommend following the same pattern for the sake of readability.

## Extending existing subsystem roles

To extend an existing role, follow the procedure outlined in the section above. Be sure to change the labels and the role name! For more information on role labels and annotations, see [Reference of role labels and annotations](/modules/user-authz/#reference-of-role-labels-and-annotations).

For example, here's how you can extend the manager role from the `deckhouse` (`d8:subsystem:deckhouse:manager`) subsystem:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: deckhouse
    rbac.deckhouse.io/capability: "custom.subsystem-capability.deckhouse.superresource_view"
  name: d8:custom:subsystem-capability:deckhouse:superresource:view
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

This way, the new capability will extend the `d8:subsystem:deckhouse:manager` role.

## Extending subsystem roles by adding a new namespace

If you need to add a new namespace (to create a namespace role binding in it by the hook), you only need to add one label:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

For more information on role labels and annotations, see [Reference of role labels and annotations](/modules/user-authz/#reference-of-role-labels-and-annotations).

This label instructs the hook to create a RoleBinding with the namespace role in this namespace:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: deckhouse
     rbac.deckhouse.io/namespace: namespace
   name: d8:custom:subsystem-capability:deckhouse:superresource:view
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

The hook watches ClusterRoleBinding objects and, when a binding is created, analyzes all system and subsystem roles to find the aggregated capabilities via the aggregation rule. It then reads the namespace from the `rbac.deckhouse.io/namespace` label and creates a RoleBinding with the namespace role in that namespace.

The hook watches only objects with the `rbac.deckhouse.io/scope: system` or `subsystem` label. A capability without that label still contributes its rules to the role through aggregation, but its `rbac.deckhouse.io/namespace` label is never read, and no RoleBinding appears in the namespace.

## Extending existing namespace roles

If the resource belongs to a namespace, you need to extend the namespace role instead of the system/subsystem role. The only difference is the labels and the name:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-namespace-as: user
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/scope: namespace
     rbac.deckhouse.io/capability: "custom.namespace-capability.mycustom.superresource_view"
   name: d8:custom:namespace-capability:mycustom:superresource:view
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

This capability will be added to the `d8:namespace:user` role.

For more information on role labels and annotations, see [Reference of role labels and annotations](/modules/user-authz/#reference-of-role-labels-and-annotations).

## Creating a custom namespace or project role

Sometimes the built-in hierarchy of levels does not fit: for example, you need a "developer" role — full view of the namespace plus reading logs, but without the right to change quotas or RBAC. Such a role is assembled from ready-made capabilities, without writing RBAC rules by hand.

The rules for custom roles:

- the name must start with `d8:custom:` (for example, `d8:custom:namespace:developer`);
- the role must carry the `rbac.deckhouse.io/kind: custom-role` label;
- a namespace or project role that will be used in a RoleBinding must also carry `rbac.deckhouse.io/delegatable: "true"`. Every user namespace is a project, and a RoleBinding there is accepted only for roles with this label. Do not put it on system or subsystem roles — the admission webhook rejects that;
- the role **cannot contain its own rules** (`rules`) — it may only aggregate capabilities via `aggregationRule`. Permissions are described in separate capabilities, so the contents of the role stay transparent;
- a single role cannot aggregate capabilities of the user-facing scopes (`namespace`, `project`) together with the administrative ones (`system`, subsystems) — such a role is rejected.

For more information on role labels and annotations, see [Reference of role labels and annotations](/modules/user-authz/#reference-of-role-labels-and-annotations).

An example: a role that includes everything allowed by `d8:namespace:viewer`, plus one specific capability (connecting to pods), selected precisely by its unique `rbac.deckhouse.io/capability` label:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:developer
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"   # Required for a RoleBinding inside a project namespace.
  annotations:
    custom.meta.deckhouse.io/title: "Developer"
    custom.meta.deckhouse.io/description: "View resources and connect to pods, without managing quotas and RBAC"
aggregationRule:
  clusterRoleSelectors:
    # Everything included in the viewer level of the namespace lineage.
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-namespace-as: viewer
    # Plus one specific capability selected by its unique name.
    - matchLabels:
        rbac.deckhouse.io/capability: "namespace-capability.kubernetes.access_terminal"
rules: []
```

If no existing capability grants the permissions you need, create your own (a `custom-capability` may contain rules) and add a selector by its `rbac.deckhouse.io/capability` label to the role's `aggregationRule` (in the example below — `matchLabels: {rbac.deckhouse.io/capability: "custom.logs-reader"}`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace-capability:logs-reader
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/capability: "custom.logs-reader"
rules:
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get", "list"]
```

To get a list of all available capabilities and their unique names, use the command:

```shell
d8 k get clusterroles -l rbac.deckhouse.io/kind=capability \
  -o custom-columns='NAME:.metadata.name,CAPABILITY:.metadata.labels.rbac\.deckhouse\.io/capability'
```

The created role is assigned exactly like a built-in one: via a RoleBinding in a namespace or via a [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) across a whole project (for project roles, use `rbac.deckhouse.io/scope: project` and aggregate `aggregate-to-project-as`). A RoleBinding in a project namespace requires the `delegatable` label on the role; a ProjectRoleBinding does not (it has its own prefix allow-list). The role cannot be assigned via a ClusterRoleBinding — just like the built-in roles of these scopes.

{% alert level="info" %}
You can also assemble such a role without YAML — with the access grant wizard in the Deckhouse Console web interface: it shows the available capabilities, builds a role out of them, and immediately creates the required binding.
{% endalert %}

## Admin level restrictions and superadmin rights

The role model provides two administration levels:

- `admin` — the everyday administrator. Manages resources, quotas, and access within their scope, but cannot perform operations that would let them break out of that scope or disrupt platform components.
- `superadmin` — the "break-glass" administrator. Has all the rights of `admin` and can additionally perform dangerous operations. Grant this level deliberately and only to those who really need it.

What is forbidden at the `admin` level and allowed only at the `superadmin` level:

- **Minting ServiceAccount tokens** (`kubectl create token`) **and making requests on behalf of a ServiceAccount** (`kubectl --as system:serviceaccount:...`). A ServiceAccount token is a ready-to-use identity: by obtaining the token of a platform component's service account, one could gain its permissions far beyond the namespace. Therefore `admin` manages the `ServiceAccount` objects themselves (create, delete) but cannot mint tokens for them or act on their behalf.
- **Modifying and deleting system resources in user namespaces.** Some platform components place their objects (for example, Dex authenticator pods, or pods and disks of virtual machines) directly in application namespaces. Such objects carry the `deckhouse.io/system-resource: "true"` label. Only `superadmin` may modify or delete them; for everyone else these operations are rejected at the API server level with an explanation.
- **Connecting to system pods** — `kubectl exec`, `kubectl attach`, and `kubectl port-forward` into a pod labelled `deckhouse.io/system-resource: "true"` are available to `superadmin` only.

The restrictions above do not apply to cluster administrators: anyone granted the standard Kubernetes `cluster-admin` role or the `SuperAdmin` [basic-model](rbac-current.html) access level, as well as members of the `system:masters`, `kubeadm:cluster-admins`, `superadmins`, and `system:sudousers` groups, keeps full break-glass access regardless of the restrictions above.

`superadmin` also has some restrictions: resources created from a [project template](/modules/multitenancy-manager/) (the `heritage: multitenancy-manager` label) cannot be modified by **anyone**, including `superadmin` and a cluster administrator — they are managed exclusively by the project controller. A role is assigned via RoleBinding and applies only in the namespace where it was granted: a `superadmin` of one namespace gets no special rights in another.

## Built-in protection mechanisms of the role model

The role model is protected by a set of checks at the API server level. They require no configuration and prevent typical mistakes and privilege escalation attempts:

- **A scoped role cannot be granted cluster-wide.** A ClusterRoleBinding to the `d8:namespace:*` or `d8:project:*` roles (and their `d8:custom:*` variants) is rejected — otherwise a role designed for one namespace or project would apply in every namespace at once. Use a RoleBinding in the desired namespace, or ProjectRoleBinding/ClusterProjectRoleBinding for a project.
- **A capability cannot be granted cluster-wide.** A ClusterRoleBinding to any capability is rejected: a capability is a building block for roles, not a standalone role. Binding a capability via a RoleBinding in a single namespace is allowed.
- **Project management cannot be obtained through a custom role.** Creating a Role or ClusterRole that grants permissions to modify project management resources (`projects`, `projecttemplates`, `projectrolebindings`, `clusterprojectrolebindings`, `projectnamespaces`) is rejected — these permissions are granted only by the built-in `d8:project:*` roles.
- **User-facing and administrative scopes cannot be mixed in one role.** A custom role cannot simultaneously aggregate capabilities of the `namespace`/`project` scopes and of the `system`/`subsystem` scopes.
- **Custom roles cannot contain direct RBAC rules** — they may only aggregate capabilities.

## Display names of roles

Every built-in role and capability carries a localized title and description in annotations:

- `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` — in Russian;
- `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` — in English.

These annotations are used, for example, by the Deckhouse Console web interface when displaying the list of roles.

If the standard title does not fit (for example, you want to name roles in your company's terms), add the `custom.meta.deckhouse.io/title` and `custom.meta.deckhouse.io/description` annotations to the role — the interface will show them instead of the standard ones. Example:

```shell
d8 k annotate clusterrole d8:namespace:admin \
  custom.meta.deckhouse.io/title='Team administrator'
```

This is the only allowed modification of built-in roles: changing their rules, aggregation, or labels is rejected.

## Deprecated role names

The previous role names of the granular model (`d8:manage:<subsystem>:<level>`, `d8:manage:all:<level>`, and `d8:use:role:<level>`) are deprecated and will be removed in a future release. For backward compatibility they are temporarily kept as alias roles: a binding to an alias aggregates the **new** role's capabilities. This is not identical to the pre-migration rights: for example, the deprecated `d8:use:role:admin` no longer grants ServiceAccount token minting or impersonation (that moved to `superadmin`).

Name mapping:

| Deprecated name | New name |
|-----------------|----------|
| `d8:manage:all:<level>` | `d8:system:<level>` |
| `d8:manage:<subsystem>:<level>` | `d8:subsystem:<subsystem>:<level>` |
| `d8:use:role:<level>` | `d8:namespace:<level>` |
| `d8:use:role:<level>:kubernetes` | `d8:namespace:<level>` (full namespace role, not kubernetes-only) |

As long as bindings to the deprecated names remain in the cluster, the `D8UserAuthzDeprecatedRBACv2RoleInUse` and `D8UserAuthzDeprecatedRBACv2CapabilityInUse` alerts fire. Migrate your existing RoleBinding and ClusterRoleBinding objects to the new role names. You can find bindings that still use the deprecated names with the following command:

```bash
d8 k get clusterrolebindings,rolebindings -A -o json \
  | jq -r '.items[] | select(.roleRef.name | test("^d8:(manage|use):")) | "\(.kind) \(.metadata.namespace // "-") \(.metadata.name) -> \(.roleRef.name)"'
```

## Getting an equivalent of the basic model's ClusterAdmin and SuperAdmin roles

There is no single-object counterpart of the [basic model's](rbac-current.html) `ClusterAdmin` and `SuperAdmin` roles in the granular model — it deliberately separates platform administration (system roles) from application access (namespace and project roles). The equivalent is assembled from **two bindings**: a ClusterRoleBinding to a system role and a [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) to a project role (the latter applies in all projects, including those created later).

Approximate level mapping:

| Basic model role | Granular model equivalent |
|-------------------|----------------------------|
| `User` | `d8:namespace:viewer` (via RoleBinding or ProjectRoleBinding) |
| `PrivilegedUser` | `d8:namespace:user` |
| `Editor` | `d8:namespace:manager` |
| `Admin` | `d8:namespace:admin` |
| `ClusterEditor` | No direct counterpart: assemble it from a ClusterProjectRoleBinding to `d8:project:manager` (all projects) plus a system role for the platform part |
| `ClusterAdmin` | `d8:system:manager` + a ClusterProjectRoleBinding to `d8:project:admin` |
| `SuperAdmin` | `d8:system:superadmin` + a ClusterProjectRoleBinding to `d8:project:superadmin` |

An example for `ClusterAdmin` (the `k8s-admins` group):

```yaml
# Platform: DP module configuration, cluster-wide resources, system namespaces.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8s-admins-platform
subjects:
  - kind: Group
    name: k8s-admins
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:system:manager
  apiGroup: rbac.authorization.k8s.io
---
# Applications: administrator in all namespaces of all projects (including future ones).
apiVersion: deckhouse.io/v1alpha3
kind: ClusterProjectRoleBinding
metadata:
  name: k8s-admins-projects
spec:
  subjects:
    - kind: Group
      name: k8s-admins
  roleRef:
    kind: ClusterRole
    name: d8:project:admin
```

For `SuperAdmin`, replace the roles with `d8:system:superadmin` and `d8:project:superadmin`.

{% alert level="info" %}
With [automatic project creation](/modules/multitenancy-manager/configuration.html#parameters-allownamespaceswithoutprojects) enabled, every user namespace is a project, so the "system role + ClusterProjectRoleBinding" pair covers both the platform and all user namespaces (except `default`, which is neither a project nor a system namespace).

You cannot create a custom "all permissions" role (`apiGroups: ["*"], resources: ["*"], verbs: ["*"]`) — it is rejected by the [built-in protection mechanisms](#built-in-protection-mechanisms-of-the-role-model). For truly unrestricted access outside the platform role model, use a ClusterRoleBinding to the built-in Kubernetes `cluster-admin` role — only someone who already has such permissions can assign it.
{% endalert %}
