---
title: "The user-authz module"
description: "Authorization and role-based access control to the resources of the Deckhouse Kubernetes Platform cluster."
---

The module generates role-based access model objects based on the standard Kubernetes RBAC mechanism. The module creates a set of cluster roles (`ClusterRole`) suitable for most user and group access management tasks.

{% alert level="warning" %}
The module provides two role-based models: the [primary](#primary-role-based-model) one (use this one) and the [legacy](#legacy-role-based-model) one, built around the `ClusterAuthorizationRule`/`AuthorizationRule` resources (its support will be discontinued in future releases).

The models are not resource-compatible — automatic conversion is impossible — but they [can be used at the same time](faq.html#can-the-legacy-and-the-primary-role-based-models-be-used-at-the-same-time): the permissions of both models are summed up.
{% endalert %}

<div style="height: 0;" id="the-new-role-based-model"></div>
<div style="height: 0;" id="experimental-role-based-model"></div>

## Primary role-based model

Unlike the [legacy DKP role-based model](#legacy-role-based-model), the primary role-based one does not use `ClusterAuthorizationRule` and `AuthorizationRule` resources. All access rights are configured in the standard Kubernetes RBAC way, i.e., by creating `RoleBinding` or `ClusterRoleBinding` resources and specifying one of the roles prepared by the `user-authz` module in them. To grant access to all namespaces of a project at once, use the [ProjectRoleBinding and ClusterProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) resources of the `multitenancy-manager` module.

> Access does not have to be granted by hand-writing YAML manifests: the Deckhouse Console web interface provides an access grant wizard. It walks you through the steps (who gets access → where → at which level), picks the right binding kind itself (`RoleBinding`, `ClusterRoleBinding`, `ProjectRoleBinding`, or `ClusterProjectRoleBinding`), and lets you assemble a custom role from ready-made building blocks without writing YAML.

The module creates special aggregated cluster roles (`ClusterRole`). By using these roles in `RoleBinding` or `ClusterRoleBinding`, you can do the following:

- Manage access to modules of a specific [subsystem](#subsystems-of-the-role-based-model).

  For example, you can use the `d8:subsystem:networking:manager` role in `ClusterRoleBinding` to allow a network administrator to configure *network* modules (such as `cni-cilium`, `ingress-nginx`, `istio`, etc.).
- Manage access to *user* resources of modules within the namespace.

  For example, the `d8:namespace:manager` role in `RoleBinding` enables deleting/creating/editing the [PodLoggingConfig](../log-shipper/cr.html#podloggingconfig) resource in the namespace. At the same time, it does not grant access to the cluster-wide [ClusterLoggingConfig](../log-shipper/cr.html#clusterloggingconfig) and [ClusterLogDestination](../log-shipper/cr.html#clusterlogdestination) resources of the `log-shipper` module, nor does it allow configuration of the `log-shipper` module itself.

### Role scopes

Every role operates in one of four scopes. The scope defines *where* the granted permissions apply and *which resource* is used to assign the role:

| Scope | Role name format | Intended for | Assigned with |
|-------|------------------|--------------|---------------|
| Namespace | `d8:namespace:<level>` | Application users (developers) | `RoleBinding` in a specific namespace |
| Project | `d8:project:<level>` | Teams working with [projects](../multitenancy-manager/) | Only [ProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) or [ClusterProjectRoleBinding](../multitenancy-manager/cr.html#clusterprojectrolebinding) |
| Subsystem | `d8:subsystem:<subsystem>:<level>` | Administrators of a part of the platform | `ClusterRoleBinding` |
| Whole platform | `d8:system:<level>` | Platform administrators | `ClusterRoleBinding` |

Access levels form a ladder: each next level includes all permissions of the previous one.

- The "namespace" and "project" scopes have five levels: `viewer` → `user` → `manager` → `admin` → `superadmin`.
- The "subsystem" and "whole platform" scopes have three levels: `viewer` → `manager` → `superadmin`. There are no `user` and `admin` levels here: the system level has no "user" resources that could be used without administering them.

The roles created by the module are divided into the following classes:

- [Namespace roles](#namespace-roles) — for assigning rights to users (such as application developers) **in a specific namespace**.
- [Project roles](#project-roles) — for assigning rights **in all namespaces of a project at once**.
- [System and subsystem roles](#system-and-subsystem-roles) — for assigning rights to administrators.

{: #rolebinding-car .anchored}

{% alert level="warning" %}
Pay attention to the specifics of configuring combined access and the use of RoleBinding and ClusterAuthorizationRule (CAR) for the same user.

If multitenancy mode is enabled in the cluster (the parameter [`enableMultiTenancy: true`](/modules/user-authz/configuration.html#parameters-enablemultitenancy)) and a ClusterAuthorizationRule (CAR) exists for the user or group specified in the RoleBinding with rules for a namespace other than the target namespace (specified in the RoleBinding), the rules from the ClusterRole specified in the RoleBinding will not apply.

This is due to the behavior of the `user-authz` module’s webhook. It checks whether a request belongs to authorized namespaces at the group level. If a user’s group is bound to a CAR with a selector limited to a specific namespace, all requests to namespaces not specified in the CAR will be rejected, regardless of whether the user has a RoleBinding for those namespaces.

It is recommended not to use RoleBinding for a user together with CAR. If combined access is required, use AuthorizationRule instead of ClusterAuthorizationRule.
{% endalert %}

<div style="height: 0;" id="use-roles"></div>

### Namespace roles

{% alert level="warning" %}
The namespace role can only be used in the `RoleBinding` resource.
{% endalert %}

Namespace roles are intended to assign rights to a user **in a specific namespace**. Users refer to, for example, developers who use a cluster configured by an administrator to deploy their applications. Such users don't need to manage DKP modules or a cluster, but they need to be able to, for example, create their Ingress resources, configure application authentication, and collect logs from applications.

The namespace role defines permissions for accessing namespaced resources of modules and standard namespaced resources of Kubernetes (`Pod`, `Deployment`, `Secret`, `ConfigMap`, etc.).

The module creates the following namespace roles:
- `d8:namespace:viewer` — allows viewing standard Kubernetes resources (except for Secrets and RBAC resources), pod logs and metrics in a specific namespace, as well as authenticating in the cluster;
- `d8:namespace:user` — in addition to the role `d8:namespace:viewer` it allows viewing secrets and RBAC resources in a specific namespace, connecting to pods (`kubectl exec`, `kubectl attach`), deleting pods (but not creating or modifying them), executing `kubectl port-forward` and `kubectl proxy`, as well as changing the number of replicas of controllers;
- `d8:namespace:manager` — in addition to the role `d8:namespace:user` it allows managing module resources (for example, `Certificate`, `PodLoggingConfig`, etc.) and standard namespaced Kubernetes resources (`Pod`, `Deployment`, `ConfigMap`, `Secret`, `Service`, `Ingress`, `NetworkPolicy`, `CronJob`, etc.) in a specific namespace;
- `d8:namespace:admin` — in addition to the role `d8:namespace:manager` it allows managing the resources `ResourceQuota`, `LimitRange`, `ServiceAccount`, `Role`, `RoleBinding` in a specific namespace;
- `d8:namespace:superadmin` — in addition to the role `d8:namespace:admin` it allows security-sensitive operations: minting ServiceAccount tokens, making requests on behalf of ServiceAccounts, and managing [system resources placed in the namespace](#admin-level-restrictions-and-superadmin-rights) (for example, Dex pods or pods/PVCs of virtual machines).

The detailed split of permissions between `admin` and `superadmin` is described [below](#admin-level-restrictions-and-superadmin-rights).

### Project roles

{% alert level="warning" %}
A project role cannot be assigned via `ClusterRoleBinding` — such an attempt is rejected. To assign a role across a whole project, use [ProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) or [ClusterProjectRoleBinding](../multitenancy-manager/cr.html#clusterprojectrolebinding); a plain `RoleBinding` in one of the project namespaces is also allowed — the role then applies in that namespace only.
{% endalert %}

Project roles (`d8:project:<level>`) are intended for working with [projects](../multitenancy-manager/) — isolated environments that may span several namespaces. The levels are the same as for namespace roles: `viewer`, `user`, `manager`, `admin`, `superadmin`.

Each project role includes all permissions of the namespace role of the same level and additionally grants permissions to manage the project itself:

- `d8:project:viewer` — the permissions of `d8:namespace:viewer` plus viewing the project's [ProjectNamespace](../multitenancy-manager/cr.html#projectnamespace) and [ProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) resources;
- `d8:project:manager` — the permissions of `d8:namespace:manager` plus managing the project's additional namespaces (`ProjectNamespace` resources);
- `d8:project:admin` — the permissions of `d8:namespace:admin` plus managing access to the project (`ProjectRoleBinding` resources) and the right to bind the built-in `d8:project:*` and `d8:namespace:*` roles (except the `superadmin` level) to other users within the project;
- `d8:project:superadmin` — analogous to the relation between `d8:namespace:superadmin` and `d8:namespace:admin`.

A role assigned via `ProjectRoleBinding` automatically applies in **all** namespaces of the project — the main one as well as the additional ones, including those created later.

<div style="height: 0;" id="manage-roles"></div>

### System and subsystem roles

{% alert level="warning" %}
The system and subsystem roles do not grant access to the namespace of user applications.

They grant access only to system namespaces (starting with `d8-` or `kube-`), and only to those system namespaces where the modules of the corresponding role subsystem are running.
{% endalert %}

System and subsystem roles are intended for assigning rights to manage the entire platform or a part of it (the [subsystem](#subsystems-of-the-role-based-model)), but not the users applications themselves. The subsystem role, for example, can allow a security administrator to manage security modules (responsible for the security functions of the cluster). Thus, the security administrator will be able to configure authentication, authorization, security policies, etc., but will not be able to manage other cluster functions (such as network and monitoring settings) or change settings in the namespaces of users applications.

The system/subsystem role defines access rights:
- to cluster-wide Kubernetes resources;
- to manage DKP modules (`moduleConfig` resource) within the [subsystem](#subsystems-of-the-role-based-model) of the role, or to all DKP modules for the role `d8:system:*`;
- to manage cluster-wide resources of DKP modules within the [subsystem](#subsystems-of-the-role-based-model) of the role, or to all resources of DKP modules for the role `d8:system:*`;
- to system namespaces (starting with `d8-` or `kube-`) in which the modules of the [subsystem](#subsystems-of-the-role-based-model) of the role operate, or to all system namespaces for the role `d8:system:*`.

The role name format is `d8:system:<ACCESS_LEVEL>` for the system roles and `d8:subsystem:<SUBSYSTEM>:<ACCESS_LEVEL>` for the subsystem roles, where:
- `SUBSYSTEM` is the role's [subsystem](#subsystems-of-the-role-based-model);
- `ACCESS_LEVEL` is the access level.

  Examples:
  - `d8:system:viewer` — access to view the configuration of all DKP modules (`moduleConfig` resource), their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except Secrets and RBAC resources) in all system namespaces (starting with `d8-` or `kube-`);
  - `d8:system:manager` — similar to the role `d8:system:viewer`, but with admin-level access, i.e., view/create/modify/delete the configuration of all DKP modules (`moduleConfig` resource), their cluster-wide resources, their namespaced resources, and standard Kubernetes objects in all system namespaces (starting with `d8-` or `kube-`);
  - `d8:subsystem:observability:viewer` — access to view the configuration of DKP modules (`moduleConfig` resource) from the `observability` area, their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except secrets and RBAC resources) in the system namespaces `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

The module provides three access levels for administrators:
- `viewer` — allows viewing standard Kubernetes resources, the configuration of modules (resources `moduleConfig`), cluster-wide resources of modules, and namespaced resources of modules in the module namespace;
- `manager` — in addition to the level `viewer` it allows managing standard Kubernetes resources, the configuration of modules (resources `moduleConfig`), cluster-wide resources of modules, and namespaced resources of modules in the module namespace;
- `superadmin` — in addition to the level `manager` it allows managing system resources of the subsystem modules.

### Subsystems of the role-based model

Each DKP module belongs to a specific subsystem. For each subsystem, there is a set of roles with different levels of access. Roles are updated automatically when the module is enabled or disabled.

For example, for the `networking` subsystem, there are the following subsystem roles that can be used in `ClusterRoleBinding`:

- `d8:subsystem:networking:viewer`
- `d8:subsystem:networking:manager`
- `d8:subsystem:networking:superadmin`

The scope of a role depends on which subsystem it belongs to:

- The scope of the `d8:system:*` roles is all system namespaces (starting with `d8-` or `kube-`) in the cluster.
- The scope of subsystem roles includes the namespaces in which the subsystem’s modules operate (see the subsystem composition table), as well as all cluster-wide objects of the subsystem’s modules.

Role-based model subsystems composition table.

{% include rbac/rbac-subsystems-list.liquid %}

### How the roles are built: aggregation and capabilities

No built-in role contains a list of permissions directly. Permissions are described in separate small cluster roles — **capabilities**. Each capability is responsible for one kind of action (for example, "view logs", "manage quotas", "connect to pods") and contains concrete RBAC rules. A role (`d8:namespace:admin`, `d8:system:viewer`, etc.) is an empty `ClusterRole` with an aggregation rule (`aggregationRule`): Kubernetes automatically collects into it the rules from all capabilities with matching labels.

Membership of objects in the role model is defined by the `rbac.deckhouse.io/*` labels — for example, the label `rbac.deckhouse.io/aggregate-to-namespace-as: admin` includes a capability into the `d8:namespace:admin` role (and, thanks to the level ladder, into all levels above). The complete list of labels and annotations is in [the reference below](#reference-of-role-labels-and-annotations).

This design has two practical consequences:

- DKP modules extend the roles automatically: when a module is enabled, its capabilities are added to the corresponding built-in roles; when it is disabled, they are removed. The permission list of a role always matches the set of enabled modules.
- You can assemble your own roles from ready-made capabilities without writing RBAC rules by hand. See [the FAQ](faq.html#how-do-i-extend-a-role-or-create-a-new-one) for how to do that.

The names of built-in roles and capabilities start with the `d8:` prefix. This namespace is reserved: you cannot create your own `ClusterRole` with a `d8:*` name — the only exception is the `d8:custom:*` prefix, which is dedicated to user-defined roles and capabilities. The labels `rbac.deckhouse.io/kind: role` and `rbac.deckhouse.io/kind: capability` are also reserved for built-in objects — use `custom-role` and `custom-capability` for your own.

### Reference of role labels and annotations

All labels the role model uses on `ClusterRole` objects:

| Label | Found on | Purpose |
|-------|----------|---------|
| `rbac.deckhouse.io/kind` | All role model objects | Object type: `role` or `capability` — built-in (reserved), `custom-role` or `custom-capability` — user-defined. Objects without this label are not processed by the role model |
| `rbac.deckhouse.io/scope` | Roles and capabilities | Scope: `namespace`, `project`, `subsystem`, `system` |
| `rbac.deckhouse.io/subsystem` | Subsystem objects | The subsystem name (for example, `networking`) — only with `scope: subsystem` |
| `rbac.deckhouse.io/aggregate-to-<scope>-as: <level>` | Capabilities and lower-level roles | The aggregation rule: includes the object into the role of the given scope and level. `<scope>` is `system`, `namespace`, `project`, or a subsystem name; `<level>` is `viewer`, `user`, `manager`, `admin`, `superadmin`. These are exactly the labels referenced by `aggregationRule` selectors |
| `rbac.deckhouse.io/capability` | Capabilities | The globally unique name of the capability (for example, `namespace-capability.kubernetes.view_logs`) — used to include the capability into a [custom role](faq.html#creating-a-custom-namespace-or-project-role) selectively |
| `rbac.deckhouse.io/use-role: <level>` | System and subsystem roles | Which namespace-role level the holder of this role automatically gets in the system namespaces of its subsystem. On the built-in roles: `viewer` → `viewer`, `manager` → `admin`, `superadmin` → `superadmin`. The access is granted by automatically created `RoleBinding` objects (labelled `rbac.deckhouse.io/automated: "true"`) |
| `rbac.deckhouse.io/namespace: <namespace>` | Capabilities | An extra namespace where a `RoleBinding` is automatically created for the holders of a system/subsystem role ([an example in the FAQ](faq.html#extending-subsystem-roles-and-adding-a-new-namespace)) |
| `rbac.deckhouse.io/delegatable: "true"` | The `d8:namespace:*`, `d8:project:*`, and user-defined roles | The role may be referenced by a `RoleBinding` [inside project namespaces](../multitenancy-manager/usage.html#which-roles-are-available-in-a-rolebinding-inside-a-project). Set it on your custom roles that should be available in projects |
| `rbac.deckhouse.io/deprecated: "true"` | [Deprecated alias roles](#deprecated-role-names) | The role is deprecated and will be removed; migrate the bindings to the new role |
| `module` | Built-in objects | The name of the DKP module the object belongs to. Convenient for aggregation selectors (for example, all capabilities of one module) |
| `heritage: deckhouse` | Built-in objects | Marks a platform object. Must not be set on your own objects |

Annotations on `ClusterRole` objects:

| Annotation | Set by | Purpose |
|------------|--------|---------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | Platform | The [display title and description](#display-names-of-roles) in Russian |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | Platform | The display title and description in English |
| `custom.meta.deckhouse.io/title`, `custom.meta.deckhouse.io/description` | Administrator | Overrides the display title/description; the only allowed modification of built-in roles |
| `rbac.deckhouse.io/bindable-only-via` | Platform | The list of binding kinds the role can be assigned with (on project roles — `ProjectRoleBinding,ClusterProjectRoleBinding`) |
| `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` | Administrator | Forbids granting the role in projects: existing bindings keep working, new ones cannot be created ([details](../multitenancy-manager/usage.html#granting-access-within-a-project)) |
| `rbac.deckhouse.io/deprecated-replaced-by` | Platform | On deprecated alias roles: the name of the role to migrate to |

### Admin level restrictions and superadmin rights

The role model deliberately separates two administration levels:

- **`admin`** — the everyday administrator. Manages resources, quotas, and access within their scope, but cannot perform operations that would let them break out of that scope or disrupt platform components.
- **`superadmin`** — the "break-glass" administrator. Has all the rights of `admin` and can additionally perform dangerous operations. Grant this level deliberately and only to those who really need it.

What is forbidden at the `admin` level and allowed only at the `superadmin` level:

- **Minting ServiceAccount tokens** (`kubectl create token`) **and making requests on behalf of a ServiceAccount** (`kubectl --as system:serviceaccount:...`). A ServiceAccount token is a ready-to-use identity: by obtaining the token of a platform component's service account, one could gain its permissions far beyond the namespace. Therefore `admin` manages the `ServiceAccount` objects themselves (create, delete) but cannot mint tokens for them or act on their behalf.
- **Modifying and deleting system resources in user namespaces.** Some platform components place their objects (for example, Dex authenticator pods, or pods and disks of virtual machines) directly in application namespaces. Such objects carry the `deckhouse.io/system-resource: "true"` label. Only `superadmin` may modify or delete them; for everyone else these operations are rejected at the API server level with an explanation.
- **Connecting to system pods** — `kubectl exec`, `kubectl attach`, and `kubectl port-forward` into a pod labelled `deckhouse.io/system-resource: "true"` are available to `superadmin` only. This protects against reading foreign secrets and interfering with platform components from inside their pods.

The `superadmin` rights are not unlimited either:

- Resources created from a [project template](../multitenancy-manager/) (the `heritage: multitenancy-manager` label) cannot be modified by **anyone**, including `superadmin` — they are managed exclusively by the project controller. To change such a resource, change the project template or the project itself.
- A role is assigned via `RoleBinding` and applies only in the namespace where it was granted: a `superadmin` of one namespace gets no special rights in another.

### Built-in protections of the role model

The role model is protected by a set of checks at the API server level. They require no configuration and prevent typical mistakes and privilege escalation attempts:

- **A scoped role cannot be granted cluster-wide.** A `ClusterRoleBinding` to the `d8:namespace:*` or `d8:project:*` roles (and their `d8:custom:*` variants) is rejected — otherwise a role designed for one namespace or project would apply in every namespace at once. Use a `RoleBinding` in the desired namespace, or `ProjectRoleBinding`/`ClusterProjectRoleBinding` for a project. A `ClusterRoleBinding` is only allowed for system and subsystem roles — those are cluster-scoped by nature.
- **A capability cannot be granted cluster-wide.** A `ClusterRoleBinding` to any capability (`d8:*-capability:*`, including custom ones) is rejected: a capability is a building block for roles, not a standalone role. Binding a capability via a `RoleBinding` in a single namespace is allowed.
- **Project management cannot be obtained through a custom role.** Creating a `Role` or `ClusterRole` that grants permissions to modify project management resources (`projects`, `projecttemplates`, `projectrolebindings`, `clusterprojectrolebindings`, `projectnamespaces`) is rejected. These permissions are granted only by the built-in `d8:project:*` roles. Without this protection, a namespace administrator could create a role with the right to create `ProjectRoleBinding` objects and grant themselves access to the whole project. Read-only roles on these resources are allowed.
- **User-facing and administrative scopes cannot be mixed in one role.** A custom role cannot simultaneously aggregate capabilities of the `namespace`/`project` scopes and of the `system`/`subsystem` scopes — this rules out a "super-role" combining access to applications and to the platform.
- **Custom roles cannot contain direct RBAC rules** — they may only aggregate capabilities. Permissions are described in separate custom capabilities, which keeps the contents of any role transparent. See [the FAQ](faq.html#how-do-i-extend-a-role-or-create-a-new-one) for details.

### Display names of roles

Every built-in role and capability carries a localized title and description in annotations:

- `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` — in Russian;
- `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` — in English.

These annotations are used, for example, by the Deckhouse Console web interface when displaying the list of roles.

If the standard title does not fit (for example, you want to name roles in your company's terms), add the `custom.meta.deckhouse.io/title` and `custom.meta.deckhouse.io/description` annotations to the role — the interface will show them instead of the standard ones. This is the only allowed modification of built-in roles: changing their rules, aggregation, or labels is rejected.

```shell
d8 k annotate clusterrole d8:namespace:admin \
  custom.meta.deckhouse.io/title='Team administrator'
```

<div style="height: 0;" id="deprecated-role-names"></div>

### Deprecated role names

The previous role names of the primary model (`d8:manage:<subsystem>:<level>`, `d8:manage:all:<level>`, and `d8:use:role:<level>`) are deprecated and will be removed in the next release. For backward compatibility they are temporarily kept as alias roles: existing bindings keep working and grant the same permissions as the new roles.

Name mapping:

| Deprecated name | New name |
|-----------------|----------|
| `d8:manage:all:<level>` | `d8:system:<level>` |
| `d8:manage:<subsystem>:<level>` | `d8:subsystem:<subsystem>:<level>` |
| `d8:use:role:<level>` | `d8:namespace:<level>` |

Note: the deprecated `d8:use:role:admin` role maps to `d8:namespace:admin` and, just like it, [no longer grants](#admin-level-restrictions-and-superadmin-rights) the right to mint ServiceAccount tokens — that now requires the `superadmin` level.

Migrate your existing `RoleBinding` and `ClusterRoleBinding` objects to the new role names. You can find bindings that still use the deprecated names with:

```shell
d8 k get clusterrolebindings,rolebindings -A -o json \
  | jq -r '.items[] | select(.roleRef.name | test("^d8:(manage|use):")) | "\(.kind) \(.metadata.namespace // "-") \(.metadata.name) -> \(.roleRef.name)"'
```

<div style="height: 0;" id="the-obsolete-role-based-model"></div>
<div style="height: 0;" id="current-role-based-model"></div>

## Legacy role-based model

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

Each next role inherits permissions from the previous roles. A role block shows only the permissions added by that role.

The list below includes:

- standard permissions from the legacy role-based model (k8s permissions);
- permissions created by Deckhouse’s built-in modules.

It does not include permissions for [modules from source](/products/kubernetes-platform/documentation/v1/architecture/module-development/run/#module-source).

When enabled in a cluster, modules from source create permissions for the resources they provide. When a module from source is disabled, the permissions it created are removed.

To view the permissions created by source modules, use the [command](#get_rules).

`verbs` aliases:
<!-- start user-authz roles placeholder -->
* read - `get`, `list`, `watch`
* read-write - `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`
* write - `create`, `delete`, `deletecollection`, `patch`, `update`

{{site.data.i18n.common.role[page.lang] | capitalize }} `User`:

```text
read:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - apps/deployments
    - apps/replicasets
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - cert-manager.io/certificaterequests
    - cert-manager.io/certificates
    - cert-manager.io/clusterissuers
    - cert-manager.io/issuers
    - cilium.io/ciliumclusterwidenetworkpolicies
    - cilium.io/ciliumnetworkpolicies
    - config.gatekeeper.sh/configs
    - configmaps
    - connection.gatekeeper.sh/connections
    - constraints.gatekeeper.sh/*
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/applications
    - deckhouse.io/awsinstanceclasses
    - deckhouse.io/azureinstanceclasses
    - deckhouse.io/clusterprojectrolebindings
    - deckhouse.io/deckhousereleases
    - deckhouse.io/deschedulers
    - deckhouse.io/dexauthenticators
    - deckhouse.io/dexclients
    - deckhouse.io/dvpinstanceclasses
    - deckhouse.io/dynamixinstanceclasses
    - deckhouse.io/gcpinstanceclasses
    - deckhouse.io/huaweicloudinstanceclasses
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/localpathprovisioners
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/nodegroups
    - deckhouse.io/openstackinstanceclasses
    - deckhouse.io/operationpolicies
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/projectnamespaces
    - deckhouse.io/projectrolebindings
    - deckhouse.io/projects
    - deckhouse.io/projecttemplates
    - deckhouse.io/securitypolicies
    - deckhouse.io/securitypolicyexceptions
    - deckhouse.io/vcdaffinityrules
    - deckhouse.io/vcdinstanceclasses
    - deckhouse.io/vsphereinstanceclasses
    - deckhouse.io/yandexinstanceclasses
    - deckhouse.io/zvirtinstanceclasses
    - discovery.k8s.io/endpointslices
    - endpoints
    - events
    - events.k8s.io/events
    - expansion.gatekeeper.sh/expansiontemplate
    - extensions.istio.io/wasmplugins
    - extensions/daemonsets
    - extensions/deployments
    - extensions/ingresses
    - extensions/replicasets
    - extensions/replicationcontrollers
    - externaldata.gatekeeper.sh/providers
    - gateway.networking.k8s.io/backendtlspolicies
    - gateway.networking.k8s.io/gatewayclasses
    - gateway.networking.k8s.io/gateways
    - gateway.networking.k8s.io/grpcroutes
    - gateway.networking.k8s.io/httproutes
    - gateway.networking.k8s.io/listenersets
    - gateway.networking.k8s.io/referencegrants
    - gateway.networking.k8s.io/tcproutes
    - gateway.networking.k8s.io/tlsroutes
    - gateway.networking.k8s.io/udproutes
    - infrastructure.cluster.x-k8s.io/deckhouseclusters
    - infrastructure.cluster.x-k8s.io/deckhousemachines
    - infrastructure.cluster.x-k8s.io/deckhousemachinetemplates
    - infrastructure.cluster.x-k8s.io/dynamixclusters
    - infrastructure.cluster.x-k8s.io/dynamixmachines
    - infrastructure.cluster.x-k8s.io/dynamixmachinetemplates
    - infrastructure.cluster.x-k8s.io/huaweicloudclusters
    - infrastructure.cluster.x-k8s.io/huaweicloudmachines
    - infrastructure.cluster.x-k8s.io/huaweicloudmachinetemplates
    - infrastructure.cluster.x-k8s.io/vcdclusters
    - infrastructure.cluster.x-k8s.io/vcdclustertemplates
    - infrastructure.cluster.x-k8s.io/vcdmachines
    - infrastructure.cluster.x-k8s.io/vcdmachinetemplates
    - infrastructure.cluster.x-k8s.io/zvirtclusters
    - infrastructure.cluster.x-k8s.io/zvirtmachines
    - infrastructure.cluster.x-k8s.io/zvirtmachinetemplates
    - limitranges
    - metrics.k8s.io/nodes
    - metrics.k8s.io/pods
    - multitenancy.deckhouse.io/availableclusterresources
    - mutations.gatekeeper.sh/assign
    - mutations.gatekeeper.sh/assignimage
    - mutations.gatekeeper.sh/assignmetadata
    - mutations.gatekeeper.sh/modifyset
    - namespaces
    - network.deckhouse.io/egressgatewaypolicies
    - network.deckhouse.io/egressgateways
    - network.deckhouse.io/metalloadbalancerbgppeers
    - network.deckhouse.io/metalloadbalancerclasses
    - network.deckhouse.io/metalloadbalancerconfigurations
    - network.deckhouse.io/metalloadbalancerpools
    - network.deckhouse.io/servicewithhealthchecks
    - networking.istio.io/destinationrules
    - networking.istio.io/gateways
    - networking.istio.io/serviceentries
    - networking.istio.io/sidecars
    - networking.istio.io/virtualservices
    - networking.istio.io/workloadentries
    - networking.istio.io/workloadgroups
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
    - security.istio.io/authorizationpolicies
    - security.istio.io/peerauthentications
    - security.istio.io/requestauthentications
    - serviceaccounts
    - services
    - status.gatekeeper.sh/configpodstatuses
    - status.gatekeeper.sh/connectionpodstatuses
    - status.gatekeeper.sh/constraintpodstatuses
    - status.gatekeeper.sh/constrainttemplatepodstatuses
    - status.gatekeeper.sh/expansiontemplatepodstatuses
    - status.gatekeeper.sh/mutatorpodstatuses
    - status.gatekeeper.sh/providerpodstatuses
    - storage.k8s.io/storageclasses
    - syncset.gatekeeper.sh/syncsets
    - telemetry.istio.io/telemetries
    - templates.gatekeeper.sh/constrainttemplates
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
write:
    - apps/deployments
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - cert-manager.io/certificates
    - cert-manager.io/issuers
    - configmaps
    - deckhouse.io/dexauthenticators
    - deckhouse.io/dexclients
    - discovery.k8s.io/endpointslices
    - endpoints
    - extensions/deployments
    - extensions/ingresses
    - gateway.networking.k8s.io/backendtlspolicies
    - gateway.networking.k8s.io/gateways
    - gateway.networking.k8s.io/grpcroutes
    - gateway.networking.k8s.io/httproutes
    - gateway.networking.k8s.io/listenersets
    - gateway.networking.k8s.io/referencegrants
    - gateway.networking.k8s.io/tcproutes
    - gateway.networking.k8s.io/tlsroutes
    - gateway.networking.k8s.io/udproutes
    - network.deckhouse.io/servicewithhealthchecks
    - networking.istio.io/destinationrules
    - networking.istio.io/gateways
    - networking.istio.io/serviceentries
    - networking.istio.io/sidecars
    - networking.istio.io/virtualservices
    - networking.istio.io/workloadentries
    - networking.istio.io/workloadgroups
    - networking.k8s.io/ingresses
    - networking.k8s.io/networkpolicies
    - persistentvolumeclaims
    - policy/poddisruptionbudgets
    - secrets
    - security.istio.io/authorizationpolicies
    - security.istio.io/peerauthentications
    - security.istio.io/requestauthentications
    - serviceaccounts
    - services
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Admin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create,patch,update:
    - pods
delete,deletecollection:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - apps/replicasets
    - cert-manager.io/certificaterequests
    - extensions/replicasets
read:
    - 'deckhouse.io/moduleconfigs (resourceNames: deckhouse)'
read-write:
    - deckhouse.io/authorizationrules
write:
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/applications
    - deckhouse.io/deckhousereleases
    - deckhouse.io/moduleconfigs
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/securitypolicyexceptions
    - extensions.istio.io/wasmplugins
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - telemetry.istio.io/telemetries
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterEditor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
delete,deletecollection:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - cert-manager.io/certificaterequests
patch,update:
    - nodes
read:
    - deckhouse.io/ingressistiocontrollers
    - deckhouse.io/istiofederations
    - deckhouse.io/istiomulticlusters
    - 'deckhouse.io/moduleconfigs (resourceNames: deckhouse)'
    - install.istio.io/istiooperators
    - multitenancy.deckhouse.io/grantableclusterresourcedefinitions
    - multitenancy.deckhouse.io/grantableclusterresourcereferences
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - sailoperator.io/istiocnis
    - sailoperator.io/istiorevisions
    - sailoperator.io/istiorevisiontags
    - sailoperator.io/istios
    - sailoperator.io/ztunnels
read-write:
    - deckhouse.io/nodegroupconfigurations
    - deckhouse.io/staticinstances
    - multitenancy.deckhouse.io/clusterresourcegrantpolicies
write:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - cert-manager.io/clusterissuers
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/applications
    - deckhouse.io/deckhousereleases
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/moduleconfigs
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/nodegroups
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/securitypolicyexceptions
    - extensions.istio.io/wasmplugins
    - extensions/daemonsets
    - gateway.networking.k8s.io/gatewayclasses
    - network.deckhouse.io/egressgatewaypolicies
    - network.deckhouse.io/egressgateways
    - storage.k8s.io/storageclasses
    - telemetry.istio.io/telemetries
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterAdmin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`):

```text
delete,deletecollection,get,list,patch,update,watch:
    - machine.sapcloud.io/alicloudmachineclasses
    - machine.sapcloud.io/awsmachineclasses
    - machine.sapcloud.io/azuremachineclasses
    - machine.sapcloud.io/gcpmachineclasses
    - machine.sapcloud.io/machinedeployments
    - machine.sapcloud.io/machines
    - machine.sapcloud.io/machinesets
    - machine.sapcloud.io/openstackmachineclasses
    - machine.sapcloud.io/packetmachineclasses
    - machine.sapcloud.io/vspheremachineclasses
    - machine.sapcloud.io/yandexmachineclasses
get,list,patch,update,watch:
    - control-plane.deckhouse.io/controlplanenodes
list:
    - dex.coreos.com/offlinesessionses
    - dex.coreos.com/passwords
patch,update:
    - deckhouse.io/vcdaffinityrules
    - infrastructure.cluster.x-k8s.io/deckhouseclusters
    - infrastructure.cluster.x-k8s.io/deckhousemachines
    - infrastructure.cluster.x-k8s.io/deckhousemachinetemplates
    - infrastructure.cluster.x-k8s.io/dynamixclusters
    - infrastructure.cluster.x-k8s.io/dynamixmachines
    - infrastructure.cluster.x-k8s.io/dynamixmachinetemplates
    - infrastructure.cluster.x-k8s.io/huaweicloudclusters
    - infrastructure.cluster.x-k8s.io/huaweicloudmachines
    - infrastructure.cluster.x-k8s.io/huaweicloudmachinetemplates
    - infrastructure.cluster.x-k8s.io/vcdclusters
    - infrastructure.cluster.x-k8s.io/vcdclustertemplates
    - infrastructure.cluster.x-k8s.io/vcdmachines
    - infrastructure.cluster.x-k8s.io/vcdmachinetemplates
    - infrastructure.cluster.x-k8s.io/zvirtclusters
    - infrastructure.cluster.x-k8s.io/zvirtmachines
    - infrastructure.cluster.x-k8s.io/zvirtmachinetemplates
    - machine.sapcloud.io/machinedeployments/scale
proxy:
    - nodes
read:
    - cluster.x-k8s.io/machinedrainrules
    - control-plane.deckhouse.io/controlplaneoperations
    - infrastructure.cluster.x-k8s.io/deckhousecontrolplanes
    - infrastructure.cluster.x-k8s.io/staticclusters
    - infrastructure.cluster.x-k8s.io/staticmachines
    - nfd.k8s-sigs.io/nodefeaturegroups
    - nfd.k8s-sigs.io/nodefeaturerules
    - nfd.k8s-sigs.io/nodefeatures
read-write:
    - cluster.x-k8s.io/clusters
    - cluster.x-k8s.io/machinedeployments
    - cluster.x-k8s.io/machinehealthchecks
    - cluster.x-k8s.io/machinepools
    - cluster.x-k8s.io/machines
    - cluster.x-k8s.io/machinesets
    - deckhouse.io/clusterauthorizationrules
    - deckhouse.io/dexproviderchecks
    - deckhouse.io/dexproviders
    - deckhouse.io/groups
    - deckhouse.io/nodeusers
    - deckhouse.io/sshcredentials
    - deckhouse.io/useroperations
    - deckhouse.io/users
    - infrastructure.cluster.x-k8s.io/staticmachinetemplates
    - nodes/configz
    - nodes/healthz
    - nodes/log
    - nodes/metrics
    - nodes/pods
    - nodes/proxy
    - nodes/stats
write:
    - cilium.io/ciliumclusterwidenetworkpolicies
    - cilium.io/ciliumnetworkpolicies
    - cluster.x-k8s.io/machinedeployments/scale
    - config.gatekeeper.sh/configs
    - connection.gatekeeper.sh/connections
    - constraints.gatekeeper.sh/*
    - deckhouse.io/awsinstanceclasses
    - deckhouse.io/azureinstanceclasses
    - deckhouse.io/clusterprojectrolebindings
    - deckhouse.io/deschedulers
    - deckhouse.io/dvpinstanceclasses
    - deckhouse.io/dynamixinstanceclasses
    - deckhouse.io/gcpinstanceclasses
    - deckhouse.io/huaweicloudinstanceclasses
    - deckhouse.io/ingressistiocontrollers
    - deckhouse.io/istiofederations
    - deckhouse.io/istiomulticlusters
    - deckhouse.io/localpathprovisioners
    - deckhouse.io/openstackinstanceclasses
    - deckhouse.io/operationpolicies
    - deckhouse.io/projectnamespaces
    - deckhouse.io/projectrolebindings
    - deckhouse.io/projects
    - deckhouse.io/projecttemplates
    - deckhouse.io/securitypolicies
    - deckhouse.io/vcdinstanceclasses
    - deckhouse.io/vsphereinstanceclasses
    - deckhouse.io/yandexinstanceclasses
    - deckhouse.io/zvirtinstanceclasses
    - expansion.gatekeeper.sh/expansiontemplate
    - externaldata.gatekeeper.sh/providers
    - install.istio.io/istiooperators
    - limitranges
    - mutations.gatekeeper.sh/assign
    - mutations.gatekeeper.sh/assignimage
    - mutations.gatekeeper.sh/assignmetadata
    - mutations.gatekeeper.sh/modifyset
    - namespaces
    - network.deckhouse.io/metalloadbalancerbgppeers
    - network.deckhouse.io/metalloadbalancerclasses
    - network.deckhouse.io/metalloadbalancerconfigurations
    - network.deckhouse.io/metalloadbalancerpools
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - resourcequotas
    - sailoperator.io/istiocnis
    - sailoperator.io/istiorevisions
    - sailoperator.io/istiorevisiontags
    - sailoperator.io/istios
    - sailoperator.io/ztunnels
    - status.gatekeeper.sh/configpodstatuses
    - status.gatekeeper.sh/connectionpodstatuses
    - status.gatekeeper.sh/constraintpodstatuses
    - status.gatekeeper.sh/constrainttemplatepodstatuses
    - status.gatekeeper.sh/expansiontemplatepodstatuses
    - status.gatekeeper.sh/mutatorpodstatuses
    - status.gatekeeper.sh/providerpodstatuses
    - syncset.gatekeeper.sh/syncsets
    - templates.gatekeeper.sh/constrainttemplates
```
<!-- end user-authz roles placeholder -->

{: #get_rules .anchored}

You can get additional list of access rules for module role from cluster ([existing user defined rules](usage.html#customizing-rights-of-high-level-roles) and non-default rules from other deckhouse modules):

```bash
D8_ROLE_NAME=Editor
kubectl get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```
