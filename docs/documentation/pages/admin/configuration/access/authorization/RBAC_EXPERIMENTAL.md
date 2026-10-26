---
title: "Granular authorization model"
permalink: en/admin/configuration/access/authorization/rbac-experimental.html
description: "Configure the granular (scope-based) RBAC authorization model in Deckhouse Platform: role scopes, access levels, project roles, and the user-authz module setup."
---

The granular role model is built on the principle of aggregation: it combines low-level roles into larger ones that cover common tasks. This makes the model easy to extend by adding your own roles.

To use the granular role model, the [`user-authz`](/modules/user-authz/) module must be enabled in the cluster. The module creates a set of special aggregated cluster roles (ClusterRole) suitable for most user and group access management tasks.

{% alert level="warning" %}
The module implements two role-based models: the granular one (this page, recommended) and the [basic](rbac-current.html) one, built around the ClusterAuthorizationRule and AuthorizationRule resources (its support will be discontinued in future releases).

The models are not resource-compatible — automatic conversion is impossible — but they can be used at the same time: the permissions of both models are summed up.
{% endalert %}

Unlike the [basic role model](rbac-current.html), the granular one does not use [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) or [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) resources. Access rights are configured the standard Kubernetes RBAC way: by creating [RoleBinding or ClusterRoleBinding](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#rolebinding-and-clusterrolebinding) resources and specifying one of the roles prepared by the `user-authz` module in them. To grant access to all namespaces of a project at once, use the [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) and [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) resources of the `multitenancy-manager` module.

{% alert level="info" %}
Access does not have to be granted by hand-writing YAML manifests: the [Deckhouse web UI](../../../../user/web/ui.html) provides an access grant wizard. It walks you through the steps (who gets access → where → at which level), picks the right binding kind itself (RoleBinding, ClusterRoleBinding, ProjectRoleBinding, or ClusterProjectRoleBinding), and lets you assemble a custom role from ready-made building blocks without writing YAML.
{% endalert %}

The module creates special aggregated cluster roles (ClusterRole). By using these roles in RoleBinding or ClusterRoleBinding, you can do the following:

- Manage access to modules of a specific [subsystem](#role-model-subsystems).

  For example, you can use the `d8:subsystem:network:manager` role in a ClusterRoleBinding to allow a network administrator to configure *network* modules (such as [`cni-cilium`](/modules/cni-cilium/), [`ingress-nginx`](/modules/ingress-nginx/), [`istio`](/modules/istio/), etc.).
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
| Namespace | `d8:namespace:<level>` | Application users (developers) | RoleBinding in a specific namespace |
| Project | `d8:project:<level>` | Teams working with [projects](/modules/multitenancy-manager/) | [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) or [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) for the whole project, RoleBinding for one namespace of the project |
| Subsystem | `d8:subsystem:<subsystem>:<level>` | Administrators of a part of the platform | ClusterRoleBinding |
| Whole platform | `d8:system:<level>` | Platform administrators | ClusterRoleBinding |

### Access levels

Each scope provides several access levels for roles:

- The "namespace" and "project" scopes have five levels: `viewer` → `user` → `manager` → `admin` → `superadmin`.
- The "subsystem" and "whole platform" scopes have three levels: `viewer` → `manager` → `superadmin`. There are no `user` and `admin` levels here: the system level has no "user" resources that could be used without administering them.

Access levels form a hierarchy with a cumulative principle: each next level inherits all permissions of the previous one and extends them. For example, the `manager` level in the namespace scope includes everything the `user` level allows, which in turn includes everything the `viewer` level allows.

## Namespace roles

{% alert level="warning" %}
A namespace role cannot be assigned through a ClusterRoleBinding. Use a RoleBinding in a namespace, or a ProjectRoleBinding or ClusterProjectRoleBinding for all namespaces of a project.
{% endalert %}

Namespace roles are intended to assign rights to a user **in a specific namespace**. Users refer to, for example, developers who use a cluster configured by an administrator to deploy their applications. Such users don't need to manage DP modules or a cluster, but they need to be able to, for example, create their Ingress resources, configure application authentication, and collect logs from applications.

A namespace role defines permissions for accessing namespaced resources of modules and standard namespaced resources of Kubernetes (Pod, Deployment, Secret, ConfigMap, etc.).

DP creates the following namespace roles:

| Role | Allowed actions |
|------|-----------------|
| `d8:namespace:viewer` | View standard Kubernetes resources (except Secrets and RBAC resources), Gateway API resources, pod logs and metrics, and authenticate in the cluster |
| `d8:namespace:user` | Everything `viewer` allows, plus: view secrets, `kubectl exec`/`attach`, delete pods (but not create or modify them), `kubectl port-forward`/`proxy`, change controller replica counts |
| `d8:namespace:manager` | Everything `user` allows, plus: manage module resources (for example, Certificate, PodLoggingConfig) and standard namespaced Kubernetes resources (Pod, Deployment, ConfigMap, Secret, Service, Ingress, NetworkPolicy, CronJob, etc.), as well as Gateway API resources (Gateway, ListenerSet, HTTPRoute, GRPCRoute, ReferenceGrant, etc.) |
| `d8:namespace:admin` | Everything `manager` allows, plus: manage ResourceQuota, LimitRange, ServiceAccount, Role, RoleBinding |
| `d8:namespace:superadmin` | Everything `admin` allows, plus security-sensitive operations: minting ServiceAccount tokens, making requests on behalf of ServiceAccounts, and managing [system resources placed in the namespace](#admin-level-restrictions-and-superadmin-rights) (for example, Dex pods or pods/PVCs of virtual machines) |

The detailed split of permissions between `admin` and `superadmin` is described in [Admin level restrictions and superadmin rights](#admin-level-restrictions-and-superadmin-rights).

### Automatic access to cluster-wide catalog resources

Working in a namespace requires reading some cluster-wide "catalogs": for example, to set `storageClassName` or `ingressClassName` in a manifest, one needs to see the list of StorageClasses and IngressClasses. Therefore, every subject that receives a RoleBinding to any `d8:namespace:*` or `d8:project:*` role automatically also gets read access to such catalog resources (StorageClass, IngressClass, PriorityClass, RuntimeClass, VolumeSnapshotClass, ClusterLogDestination, etc.).

Technically, this looks like an automatically created ClusterRoleBinding to the [`d8:dict`](#global-resource-dictionaries) role with the `rbac.deckhouse.io/dict: "true"` label. Such objects are managed by the platform: they appear when the subject is granted their first namespace binding and are removed once none are left — there is no need to edit them manually.

## Project roles

{% alert level="warning" %}
A project role cannot be assigned via ClusterRoleBinding — such an attempt is rejected. To assign a role across a whole project, use [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) or [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). A plain RoleBinding in one of the project namespaces is also allowed — the role then applies in that namespace only.
{% endalert %}

Project roles (`d8:project:<level>`) are intended for working with [projects](/modules/multitenancy-manager/) — isolated environments that may span several namespaces. The levels are the same as for namespace roles: `viewer`, `user`, `manager`, `admin`, `superadmin`.

Each project role includes all permissions of the namespace role of the same level and additionally grants permissions to manage the project itself:

- `d8:project:viewer` — the permissions of `d8:namespace:viewer` plus viewing the project's ProjectNamespace and ProjectRoleBinding resources;
- `d8:project:user` — the permissions of `d8:namespace:user` plus viewing the project's ProjectNamespace and ProjectRoleBinding resources;
- `d8:project:manager` — the permissions of `d8:namespace:manager` plus managing the project's additional namespaces (ProjectNamespace resources);
- `d8:project:admin` — the permissions of `d8:namespace:admin` plus managing access to the project (ProjectRoleBinding resources) and the right to bind the built-in `d8:project:*` and `d8:namespace:*` roles (except the `superadmin` level) to other users within the project;
- `d8:project:superadmin` — analogous to the relation between `d8:namespace:superadmin` and `d8:namespace:admin`.

A role assigned via ProjectRoleBinding automatically applies in **all** namespaces of the project — the main one as well as the additional ones, including those created later.

## System and subsystem roles

{% alert level="warning" %}
System and subsystem roles do not grant access to the namespaces of user applications.

They grant access only to system namespaces (starting with `d8-` or `kube-`), and only to those system namespaces where the modules of the corresponding role subsystem are running.

Whether a role lets you grant access to other people is described in [Who can assign roles](#who-can-assign-roles).
{% endalert %}

System and subsystem roles are intended for assigning rights to manage the entire platform or a part of it (the [subsystem](#role-model-subsystems)), but not the user applications themselves. The subsystem role, for example, can allow a security administrator to manage security modules (responsible for the security functions of the cluster). Thus, the security administrator will be able to configure security policies, certificate issuance, etc., but will not be able to manage other cluster functions (such as access management, network and monitoring settings) or change settings in the namespaces of user applications.

{% alert level="warning" %}
A system/subsystem role limits which modules and namespaces a subject can address, but it does not limit the privileges a subject can obtain through the modules it is allowed to manage. This matters most for the `iam` subsystem: the right to manage authentication and authorization is equivalent to full control over the cluster — a subject that can manage the `user-authn` module can register an identity provider or reset the credentials of any local user, and a subject that can manage the `user-authz` module can modify authorization rules. In both cases, they can obtain an identity with any privileges, including cluster administrator, so treat an `iam` subsystem role as a cluster administrator role when planning access.
{% endalert %}

The system/subsystem role defines access rights:

- to cluster-wide Kubernetes resources;
- to manage DP modules (ModuleConfig resource) within the [subsystem](#role-model-subsystems) of the role, or to all DP modules for the role `d8:system:*`;
- to manage cluster-wide resources of DP modules within the [subsystem](#role-model-subsystems) of the role, or to all resources of DP modules for the role `d8:system:*`;
- to read the namespaces of the modules of the role's [subsystem](#role-model-subsystems), or of all DP modules for the role `d8:system:*`; writing there is reserved for the `superadmin` level (see [Access of system and subsystem roles to the namespaces of the modules](#access-of-system-and-subsystem-roles-to-the-namespaces-of-the-modules)).

The role name format is `d8:system:<ACCESS_LEVEL>` for the system roles and `d8:subsystem:<SUBSYSTEM>:<ACCESS_LEVEL>` for the subsystem roles, where:

- `SUBSYSTEM` is the role's [subsystem](#role-model-subsystems);
- `ACCESS_LEVEL` is the access level.

Role examples:

- `d8:system:viewer` — access to view the configuration of all DP modules (ModuleConfig resource), their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except Secrets and RBAC resources) in the namespaces of all DP modules;
- `d8:system:manager` — similar to `d8:system:viewer`, but with the right to view/create/modify/delete the configuration of all DP modules (ModuleConfig resource) and their cluster-wide resources; in the namespaces of DP modules it keeps the read access of `d8:system:viewer`;
- `d8:subsystem:observability:viewer` — access to view the configuration of DP modules (ModuleConfig resource) from the `observability` subsystem, their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except Secrets and RBAC resources) in the system namespaces `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

DP has three access levels for system and subsystem roles:

- `viewer` — allows viewing standard Kubernetes resources, the configuration of modules (ModuleConfig resources), cluster-wide resources of modules, and namespaced resources of modules in the module namespace;
- `manager` — in addition to `viewer`, allows managing the configuration of modules (ModuleConfig resources) and the cluster-wide resources of modules; in the module namespaces it grants the same read access as `viewer`;
- `superadmin` — in addition to `manager`, allows everything in the module namespaces: managing standard Kubernetes resources and the namespaced resources of modules, reading Secrets, connecting to pods, issuing ServiceAccount tokens, and managing the system resources of the subsystem modules.

### Access of system and subsystem roles to the namespaces of the modules

The holder of a system or subsystem role automatically gets a RoleBinding to a namespace role in every system namespace in which the modules of the role's subsystem run (a `d8:system:*` role covers the namespaces of all DP modules): the `viewer` and `manager` levels get `d8:namespace:viewer`, the `superadmin` level gets `d8:namespace:superadmin`. The platform creates and removes these RoleBindings itself (they carry the `rbac.deckhouse.io/automated: "true"` label), one per ClusterRoleBinding of the role.

The namespaces of the modules belong to the platform. They hold ServiceAccounts with cluster-wide rights (the Deckhouse controller among them) and the credentials of the modules, so any write there, and any read of a Secret there, is in effect the right to act as those ServiceAccounts anywhere in the cluster. This is why a `manager` configures its subsystem through ModuleConfig objects and the cluster-wide resources of the modules, and in the module namespaces only views objects, logs, metrics, and events. A `manager` cannot read Secrets, run `kubectl exec`, `attach`, or `port-forward`, delete or scale pods, or create anything there. A `superadmin` can do all of that. Grant this level deliberately and to a named subject: it is the emergency path into the platform, and its actions are visible in the audit log. Do not use impersonation of system users (`--as`) as a substitute.

When working with system and subsystem roles, keep in mind the following:

- A RoleBinding to `d8:namespace:user` or a higher level in a system namespace, created by hand, grants the subject the rights of every ServiceAccount of that namespace. The same holds for the `allowAccessToSystemNamespaces: true` option of a ClusterAuthorizationRule of the [basic role model](rbac-current.html): with it, the `Admin` and `ClusterAdmin` access levels are equivalent to a cluster administrator.
- The system namespaces are those the `multitenancy-manager` module never turns into projects: `d8-*`, `kube-*`, `upmeter-*`, `default`, and the namespaces labeled `heritage: deckhouse` or `heritage: upmeter`.
- A custom system or subsystem role (`d8:custom:*`) is projected as `viewer` whatever its `rbac.deckhouse.io/use-role` label says: the webhook refuses a higher value, and a role created with one earlier is projected as `viewer` with a `ProjectionCapped` event on the role.

### Who can assign roles

A system or subsystem role does not, by itself, let you grant access to other people.

Creating a User or Group that is not already a grant subject is ordinary object creation.

Creating a User for an email that already carries a grant, writing or deleting a ClusterAuthorizationRule, or resetting that identity through UserOperation, is granting roles. The request is admitted only if the requester already has covering permissions or is explicitly allowed to assign those roles.

Connecting a DexProvider is measured the same way against the identities the provider can assert: [`spec.allowedIdentities`](/modules/user-authn/cr.html#dexprovider-v1-spec-allowedidentities) (emails, email domains, groups) and the provider's own group filters bound that set, and only the roles already granted inside it count. A provider without an email limit can assert every email, including the ones that hold `SuperAdmin`, so writing it requires `SuperAdmin`. Basic Authentication of an OIDC or a Crowd provider bypasses Dex and its limits, so such a provider counts as one without an email limit, and an OIDC one also without a group limit; a Crowd provider keeps only `crowd.groups` on the group axis. Details and the list of changes admitted without a check are in the [`user-authn` module documentation](/modules/user-authn/usage.html).

The ClusterAuthorizationRule `spec.accessLevel` field is a [basic model](rbac-current.html) level: `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, `ClusterAdmin`, `SuperAdmin`. An `iam` subsystem manager and a holder of `d8:system:manager` assign the levels `ClusterEditor` includes: `User`, `PrivilegedUser`, `Editor`, and `ClusterEditor`, but not `Admin`, `ClusterAdmin`, or `SuperAdmin`, and neither the Kubernetes `cluster-admin` ClusterRole.

A ClusterAuthorizationRule that opens the system namespaces counts as a `ClusterAdmin` assignment whatever its `accessLevel`, so these managers cannot write it. Such a rule is a rule with `allowAccessToSystemNamespaces: true`, with `namespaceSelector.matchAny`, with a `namespaceSelector.labelSelector` that does not name its namespaces by the `kubernetes.io/metadata.name` label, a rule for a subject that another rule already lets into them (the rules of a subject add up), or any rule while multitenancy mode is disabled. Without multitenancy mode, as in DP Open/CE, these managers therefore cannot write ClusterAuthorizationRule objects at all.

The rules of a request add up: those of its user, of every group in its token, and of its ServiceAccount. A rule therefore also opens the system namespaces when another rule already lets in one of the requests it applies to:

- a local user it names or a member of its groups at any depth of nesting, with the groups that user is in and the groups above them;
- the requester, with the groups of its own token;
- any request, through `system:authenticated` (a ServiceAccount also through `system:serviceaccounts` and `system:serviceaccounts:<namespace>`).

A rule for `system:authenticated` applies to every request, so it opens the system namespaces as soon as any other rule opens them. A rule for `system:serviceaccounts` does so as soon as another rule lets in a ServiceAccount, and a rule for `system:serviceaccounts:<namespace>` as soon as another rule lets in a ServiceAccount of that namespace. A User subject named `system:serviceaccount:<namespace>:<name>` is that ServiceAccount.

A member of a Group gets the rules of every group above it, so writing a Group needs the range for their roles too. For the same reason, these managers cannot add a member that another rule already lets into the system namespaces to a Group that has a ClusterAuthorizationRule, or is in a group that has one. The groups an identity provider asserts for other users are unknown to the webhook and are not taken into account.

For the same reason, a user or a group that such a rule names counts as a `ClusterAdmin` target: these managers cannot create a User for its email, add members to such a Group, run a UserOperation on such a user, or connect a DexProvider that can assert such an identity. Turning multitenancy mode off in the `user-authz` ModuleConfig opens the system namespaces to every rule and requires the `ClusterAdmin` range as well. Within the granular model, the IAM manager assigns the `iam` subsystem roles up to `d8:subsystem:iam:manager`, and the system manager assigns the roles of any scope up to the `manager` level.

These managers do not write admission webhooks and policies (ValidatingWebhookConfiguration, MutatingWebhookConfiguration, ValidatingAdmissionPolicy, and ValidatingAdmissionPolicyBinding): the superadmins of the `security` and `cluster` subsystems and `d8:system:superadmin` do. A mutating webhook can change any object the platform creates, its own pods included, and removing a webhook or a policy switches off what it enforces, such as the limits above. The `d8-deckhouse-validating-webhook-handler-*` ValidatingWebhookConfigurations, which hold the webhooks of the Deckhouse modules, can be created, changed, or deleted only by `webhook-handler`, the platform components, and the superadmins, including the holders of a role that grants every verb on everything.

A requester with every permission across the cluster counts as `SuperAdmin` in these checks and can assign any role, including `SuperAdmin` and `cluster-admin`. For example, a subject of the Kubernetes `cluster-admin` ClusterRole has such permissions if the role is granted by a ClusterRoleBinding or by a ClusterAuthorizationRule without `namespaceSelector` and `limitNamespaces`. If multitenancy mode is enabled, such a rule reaches the system namespaces only with `namespaceSelector.matchAny: true` (or the deprecated `allowAccessToSystemNamespaces: true`).

Permissions from a ClusterAuthorizationRule with `namespaceSelector` or `limitNamespaces` do not count toward this, even if the rule grants `cluster-admin`. Any non-empty `limitNamespaces` counts as a limit on purpose, even if its entries match every namespace, such as `.*` together with `allowAccessToSystemNamespaces: true`. Such a rule still counts through its `accessLevel`. For example, `accessLevel: SuperAdmin` gives the `SuperAdmin` range whatever the namespace limits of the rule.

`limitNamespaces` and `namespaceSelector` restrict access to namespaced resources only. A subject with the `ClusterAdmin` or `SuperAdmin` access level can still create ClusterRoleBinding and ClusterAuthorizationRule objects, so these fields do not confine what it can grant across the cluster. Give these levels only to subjects that may hold access to the whole cluster. A ClusterAuthorizationRule counts only through the bindings that `user-authz-controller` has created for it.

Webhook deny messages call the `accessLevel` values basic, so they are not confused with the granular role levels (`viewer`, `manager`, `superadmin`).

Permission to create User and Group objects in the `user-authn` module is not enough: it does not grant the roles already attached to that email.

## Global resource dictionaries

In addition to the roles and capabilities of the granular model, the module creates a special ClusterRole `d8:dict`. It grants read access to cluster-wide "reference" resources that users often need to view when creating objects. For example, a user creating a PersistentVolumeClaim needs to see the available StorageClasses, and a user creating an Ingress needs to see the IngressClasses.

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

`user-authz-controller` keeps one ClusterRoleBinding to `d8:dict` for every subject that holds one of the following RoleBindings:

- a RoleBinding to a `d8:namespace:*` or `d8:project:*` role, including the RoleBindings that `multitenancy-manager` creates from a ProjectRoleBinding or a ClusterProjectRoleBinding;
- a RoleBinding that the module creates for an AuthorizationRule (the `User`, `PrivilegedUser`, `Editor`, and `Admin` levels).

The `system:multitenancy-manager:*` users, which `multitenancy-manager` acts as, get none. This means that project users can discover reference resources without any manual configuration. The binding is removed when the subject has no such RoleBinding left. You do not create it yourself.

{% alert level="info" %}
The `d8:dict` role is independent of the `multitenancy-manager` cluster-wide resource access management mechanism: it only gives **read** access to reference resources so users can discover them. The mechanism controls **which resource values** a project may actually reference when creating objects.
{% endalert %}

## Role model subsystems

Each DP module belongs to a specific subsystem. For each subsystem, there is a set of roles with different levels of access. Roles are updated automatically when the module is enabled or disabled.

The subsystems of the platform are `iam`, `security`, `cluster`, `delivery`, `network`, `storage`, `observability`, and `managed-services`.

For example, for the `network` subsystem, there are the following subsystem roles that can be used in [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/):

- `d8:subsystem:network:viewer`;
- `d8:subsystem:network:manager`;
- `d8:subsystem:network:superadmin`.

The scope of a role depends on which subsystem it belongs to:

- The scope of the `d8:system:*` roles is the namespaces of all enabled DP modules, that is, the namespaces that their capabilities name in the `rbac.deckhouse.io/namespace` label.
- The scope of subsystem roles includes the namespaces in which the subsystem's modules operate (see the subsystem composition table below), as well as all cluster-wide objects of the subsystem's modules.

Role model subsystems composition table.

{% include rbac/rbac-subsystems-list.liquid %}

## How the roles are built: aggregation and capabilities

No built-in role contains a list of permissions directly. Permissions are described in separate small cluster roles — **capabilities**. Each capability is responsible for one kind of action (for example, "view logs", "manage quotas", "connect to pods") and contains concrete RBAC rules. A role (`d8:namespace:admin`, `d8:system:viewer`, etc.) is an empty ClusterRole with an aggregation rule (`aggregationRule`): Kubernetes automatically collects into it the rules from all capabilities with matching labels.

This design has two practical consequences:

- DP modules extend the roles automatically: when a module is enabled, its capabilities are added to the corresponding built-in roles; when it is disabled, they are removed. The permission list of a role always matches the set of enabled modules.
- You can assemble your own roles from ready-made capabilities without writing RBAC rules by hand, as shown below.

The names of built-in roles and capabilities start with the `d8:` prefix. This prefix is reserved: you cannot create your own ClusterRole with a `d8:*` name — the only exception is the `d8:custom:*` prefix, which is dedicated to user-defined roles and capabilities.

## Creating a new subsystem role

If, for example, the current subsystems do not fit the role distribution in the company, you need to create a new [subsystem](#role-model-subsystems)
that includes roles from the `cluster` subsystem, the `network` subsystem, and the `user-authn` module.

To meet this need, create the following role:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: viewer
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-cluster-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-network-as: manager
    - matchLabels:
        rbac.deckhouse.io/scope: system
        module: user-authn
rules: []
```

The labels for the new role listed at the top suggest that:

- The namespace role that the subjects of the role get in the module namespaces (`user-authz-controller` creates the RoleBinding objects); a custom role is projected as `viewer` at most:

  ```yaml
  rbac.deckhouse.io/use-role: viewer
  ```

- The role is a custom role (custom roles never define their own rules, they only aggregate capabilities):

  ```yaml
  rbac.deckhouse.io/kind: custom-role
  ```

  {% alert level="info" %}
  This label is mandatory.
  {% endalert %}

- The role is a subsystem one, and it shall be handled accordingly:

  ```yaml
  rbac.deckhouse.io/scope: subsystem
  ```

- There is a subsystem for which the role is responsible:

  ```yaml
  rbac.deckhouse.io/subsystem: mycustom
  ```

Then there are selectors that implement aggregation:

- This one aggregates the manager role from the `cluster` subsystem:

  ```yaml
  rbac.deckhouse.io/aggregate-to-cluster-as: manager
  ```

- This one aggregates all the system-scope capabilities defined for the `user-authn` module:

  ```yaml
   rbac.deckhouse.io/scope: system
   module: user-authn
  ```

{% alert level="info" %}
For more information on role labels and annotations, see [Reference of role labels and annotations](/modules/user-authz/#reference-of-role-labels-and-annotations).
{% endalert %}

This way, your role will combine the permissions of the `cluster` subsystem, `network` subsystem, and the `user-authn` module.

Specifics of using custom roles:

- Custom roles and capabilities must be named with the `d8:custom:` prefix (all other names with the `d8:` prefix are reserved for DP built-in objects). The name must agree with the declared scope: a subsystem role is `d8:custom:<subsystem>:<name>` (the segment is the subsystem itself, as in the example above), a namespace or project role is `d8:custom:namespace:<name>` or `d8:custom:project:<name>`, and a capability is `d8:custom:<scope>-capability:<name>`. A name that disagrees with the `rbac.deckhouse.io/scope` label is rejected.
- RoleBindings with a namespace role (`d8:namespace:<level>`) will be created in the namespaces of the aggregated subsystems' modules; the level is specified by the `rbac.deckhouse.io/use-role` label, `viewer` at most for a custom role. Without the label, no such RoleBinding is created.
- A custom role cannot be aggregated into `d8:system:<level>`, because only a custom capability may carry the `rbac.deckhouse.io/aggregate-to-system-as` label. To give the role to the holders of `d8:system:manager`, bind it to them with a ClusterRoleBinding.

## Extending a custom role

Suppose a new cluster CRD object, MySuperResource, has been created in the cluster (a subsystem role example), and you need to extend the custom role from the example above to include the permissions to interact with this resource. Follow these steps:

1. Add a new selector to the role:

   ```yaml
   rbac.deckhouse.io/aggregate-to-mycustom-as: manager
   ```

   For more information on role labels and annotations, see [Reference of role labels and annotations](/modules/user-authz/#reference-of-role-labels-and-annotations).

   This selector enables capabilities to be aggregated to the new subsystem by specifying this label. After adding the new selector, the role looks as follows:

    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: d8:custom:mycustom:manager
      labels:
        rbac.deckhouse.io/use-role: viewer
        rbac.deckhouse.io/kind: custom-role
        rbac.deckhouse.io/scope: subsystem
        rbac.deckhouse.io/subsystem: mycustom
    aggregationRule:
      clusterRoleSelectors:
        - matchLabels:
            rbac.deckhouse.io/aggregate-to-cluster-as: manager
        - matchLabels:
            rbac.deckhouse.io/aggregate-to-network-as: manager
        - matchLabels:
            rbac.deckhouse.io/scope: system
            module: user-authn
        - matchLabels:
            rbac.deckhouse.io/aggregate-to-mycustom-as: manager
    rules: []
    ```

1. Create a new capability that defines the permissions for the new resource. For example, read-only access:

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

   The capability adds its permissions to the subsystem role, granting the right to view the new object.

Specifics:

- Name a custom capability `d8:custom:<SCOPE>-capability:<NAME>`, where `<SCOPE>` is the value of its `rbac.deckhouse.io/scope` label. A name that disagrees with the label is rejected.

## Extending existing subsystem roles

To extend an existing role, follow the same steps as in the section above, but change the labels and the role name. For more information on role labels and annotations, see [Reference of role labels and annotations](/modules/user-authz/#reference-of-role-labels-and-annotations).

For example, here's how you can extend the manager role from the `cluster` (`d8:subsystem:cluster:manager`) subsystem:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-cluster-as: manager
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: cluster
    rbac.deckhouse.io/capability: "custom.subsystem-capability.cluster.superresource_view"
  name: d8:custom:subsystem-capability:cluster:superresource:view
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

This way, the new capability will extend the `d8:subsystem:cluster:manager` role.

## Extending subsystem roles by adding a new namespace

The namespaces where the holders of a system or subsystem role get a RoleBinding with the namespace role are the namespaces of its modules. `user-authz-controller` takes them from the `rbac.deckhouse.io/namespace` label of the system capabilities Deckhouse ships, which have a `d8:` name and the `heritage: deckhouse` label. On a custom capability, the label is ignored: the capability adds its rules to the roles that aggregate it, but no namespace.

To give the subjects of such a role access to another namespace, bind them to a namespace role there with a RoleBinding:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cluster-managers-viewer
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:namespace:viewer
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: cluster-managers
```

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

Sometimes the built-in hierarchy of levels does not fit: for example, you need a "developer" role — full view of the namespace plus terminal access to pods (`exec`, `attach`), but without the right to change quotas or RBAC. Such a role is assembled from ready-made capabilities, without writing RBAC rules by hand.

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

If no existing capability grants the permissions you need, create your own (a `custom-capability` may contain rules) and add a selector by its `rbac.deckhouse.io/capability` label to the role's `aggregationRule` (in the example below — `matchLabels: {rbac.deckhouse.io/capability: "custom.namespace-capability.port-forward"}`). The label value of a custom capability is `custom.<SCOPE>-capability.<NAME>`, and a role that is bound in a project may select only the `namespace-capability` and `project-capability` scopes:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace-capability:port-forward
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/capability: "custom.namespace-capability.port-forward"
rules:
  - apiGroups: [""]
    resources: ["pods/portforward"]
    verbs: ["create"]
```

To get a list of all available capabilities and their unique names, use the command:

```shell
d8 k get clusterroles -l rbac.deckhouse.io/kind=capability -o custom-columns='NAME:.metadata.name,CAPABILITY:.metadata.labels.rbac\.deckhouse\.io/capability'
```

The created role is assigned exactly like a built-in one: via a RoleBinding in a namespace or via a [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) across a whole project (for project roles, use `rbac.deckhouse.io/scope: project` and aggregate `aggregate-to-project-as`). A RoleBinding in a project namespace requires the `delegatable` label on the role; a ProjectRoleBinding does not (it has its own prefix allow-list). The role cannot be assigned via a ClusterRoleBinding — just like the built-in roles of these scopes.

{% alert level="info" %}
You can also assemble such a role without YAML — with the access grant wizard in the Deckhouse web UI: it shows the available capabilities, builds a role out of them, and immediately creates the required binding.
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
- **A namespace or project role cannot include platform capabilities.** A custom role with `scope: namespace`, `scope: project`, or no scope may not select system or subsystem capabilities by the `rbac.deckhouse.io/capability` label, nor the `system` or a subsystem scope by the `rbac.deckhouse.io/aggregate-to-<scope>-as` label. No custom role may combine the capabilities of the `namespace`/`project` scopes with those of the `system`/`subsystem` scopes.
- **A custom capability carries rules and cannot have an `aggregationRule`.**
- **The name must match the scope**: `d8:custom:namespace:<name>`, `d8:custom:project:<name>`, `d8:custom:system:<name>`, `d8:custom:<subsystem>:<name>`, and `d8:custom:<scope>-capability:<name>` for a capability.
- **Only a custom capability may carry an `rbac.deckhouse.io/aggregate-to-<scope>-as` label of a platform scope.**
- **The `rbac.deckhouse.io/delegatable: "true"` label is accepted only on a role that aggregates only namespace and project capabilities.**
- **Custom roles cannot contain direct RBAC rules** — they may only aggregate capabilities.
- **The `rbac.deckhouse.io/kind: role` and `rbac.deckhouse.io/kind: capability` labels are reserved for DP built-in objects.** A ClusterRole with one of these labels is rejected. For your own objects, use `custom-role` and `custom-capability`.

## Display names of roles

Every built-in role and capability carries a localized title and description in annotations:

- `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` — in Russian;
- `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` — in English.

These annotations are used, for example, by the Deckhouse web UI when displaying the list of roles.

If the standard title does not fit (for example, you want to name roles in your company's terms), add the `custom.meta.deckhouse.io/title` and `custom.meta.deckhouse.io/description` annotations to the role — the interface will show them instead of the standard ones. Example:

```shell
d8 k annotate clusterrole d8:namespace:admin custom.meta.deckhouse.io/title='Team administrator'
```

This is the only allowed modification of built-in roles: changing their rules, aggregation, or labels is rejected.

## Deprecated role names

The previous role names of the granular model (`d8:manage:<subsystem>:<level>`, `d8:manage:all:<level>`, and `d8:use:role:<level>`) are deprecated and will be removed in the next release. For backward compatibility they are temporarily kept as alias roles. A binding to an alias aggregates the **new** role's capabilities, except where the list of subsystems changed (see [below](#the-subsystem-list-of-the-aliases)).

The rights of an alias are not identical to the pre-migration ones. The deprecated `d8:use:role:admin` no longer grants ServiceAccount token minting or impersonation, which moved to `superadmin`. The `d8:manage:*` aliases are not `delegatable`. The `d8:use:role:*` aliases are, so a RoleBinding to them inside a project namespace keeps working, but a new ProjectRoleBinding or ClusterProjectRoleBinding to an old name is refused.

Name mapping:

| Deprecated name | New name |
|-----------------|----------|
| `d8:manage:all:<level>` | `d8:system:<level>` |
| `d8:manage:deckhouse:<level>`, `d8:manage:infrastructure:<level>`, `d8:manage:kubernetes:<level>` | `d8:subsystem:cluster:<level>`, which grants more. The modules that moved to other subsystems come with the roles of those subsystems |
| `d8:manage:networking:<level>` | `d8:subsystem:network:<level>`, which also grants the rights for `keepalived` |
| `d8:manage:security:<level>` | `d8:subsystem:security:<level>` together with `d8:subsystem:iam:<level>`, which grants the rights for `user-authz` and `user-authn` and more |
| `d8:manage:observability:<level>` | `d8:subsystem:observability:<level>` |
| `d8:manage:storage:<level>` | `d8:subsystem:storage:<level>`, which also grants the rights for `local-path-provisioner` |
| `d8:use:role:<level>` | `d8:namespace:<level>` |
| `d8:use:role:<level>:kubernetes` | `d8:namespace:<level>` (full namespace role, not kubernetes-only) |

<div style="height: 0;" id="the-subsystem-list-of-the-aliases"></div>

The list of subsystems changed together with the role names. `deckhouse`, `infrastructure`, and `kubernetes` were merged into `cluster`, which also took over modules from other subsystems, `networking` was renamed to `network`, and `user-authz`, `user-authn`, and `multitenancy-manager` moved to `iam`.

The aliases of the `deckhouse`, `infrastructure`, and `kubernetes` roles therefore do not aggregate the whole `cluster` subsystem. They select the capabilities of the platform modules that were in the old subsystem, wherever those modules are now. `d8:subsystem:cluster:<level>` grants more, so when you recreate such a binding, check which subsystem roles its subjects need.

The alias of `d8:manage:security:<level>` also keeps the rights for `user-authz` and `user-authn`, which now come with `d8:subsystem:iam:<level>`. That role also grants the rights for `multitenancy-manager` and, at the `manager` level, the right to assign roles, which `d8:subsystem:security:<level>` does not grant. So a binding to `d8:manage:security:<level>` is replaced by two bindings with the same subjects, to `d8:subsystem:iam:<level>` and to `d8:subsystem:security:<level>`. The `rbac.deckhouse.io/deprecated-replaced-by` annotation of the alias names only the second one.

The aliases of `networking` and `storage` leave out `keepalived` and `local-path-provisioner`, which moved to `network` and `storage` from `infrastructure`, so the new roles grant more than these aliases.

The `rbac.deckhouse.io/deprecated-replaced-by` annotation of an alias names its main replacement, and the description of the alias names the roles of the other subsystems its modules moved to. These roles cover whole subsystems. To grant the rights for some modules only, create a custom role that selects their system capabilities by the `rbac.deckhouse.io/capability` label, for example `system-capability.multitenancy-manager.view` (see [Creating a new subsystem role](#creating-a-new-subsystem-role)).

The aliases of the `deckhouse`, `infrastructure`, `kubernetes`, and `networking` roles and `d8:manage:all:<level>` also select the labels of these old subsystems, such as `rbac.deckhouse.io/aggregate-to-deckhouse-as`. Modules released on their own and custom capabilities may still carry them. No role of the new model collects these labels, so such capabilities reach the holders of the old names only, and the `D8UserAuthzRetiredAggregationLabel` alert names them.

As long as bindings to the deprecated names remain in the cluster, the `D8UserAuthzDeprecatedRBACv2RoleInUse` (a binding to an alias role) and `D8UserAuthzDeprecatedRBACv2CapabilityInUse` (a binding to a deprecated name that no ClusterRole has, such as a capability of the previous release, so the binding grants nothing) alerts fire. The release that removes the aliases carries the `deprecatedRBACv2BindingsCount` release requirement: the upgrade stays pending, with the offending bindings listed in the DeckhouseRelease status, until every binding to an alias role is recreated on a new role name. A binding that grants nothing does not hold the release.

Migrate your existing RoleBinding and ClusterRoleBinding objects to the new role names:

- A ClusterRoleBinding named `user-authz:<rule>:additional-role:<role>` is rendered by `user-authz-controller` from `spec.additionalRoles` of the ClusterAuthorizationRule `<rule>`, and the controller creates it again when it is deleted. Change the role name in `spec.additionalRoles` of that rule, not the binding.
- A binding that a project template renders is changed in the ProjectTemplate, because `multitenancy-manager` renders it again from the template.

You can find bindings that still use the deprecated names, without the RoleBindings that `user-authz-controller` keeps for them, with the following command:

```bash
d8 k get clusterrolebindings,rolebindings -A -o json \
  | jq -r '.items[] | select(.roleRef.name | test("^d8:(manage|use):")) | select(.metadata.labels["rbac.deckhouse.io/automated"] != "true") | "\(.kind) \(.metadata.namespace // "-") \(.metadata.name) -> \(.roleRef.name)"'
```

### Custom roles of the previous scheme

{% alert level="warning" %}
The built-in capabilities no longer carry the `rbac.deckhouse.io/kind: manage` and `rbac.deckhouse.io/kind: use` labels. A custom role of the previous scheme that selects capabilities by these labels has lost their permissions, and so have its subjects. Custom roles get no compatibility aliases.
{% endalert %}

The `D8UserAuthzLegacyRBACv2CustomRoleFound` alert lists such roles. How to find and repair them, along with the mapping of the previous labels to the new ones, is described in the [`user-authz` FAQ section on migrating custom roles to the new scheme](/modules/user-authz/faq.html#how-do-i-migrate-custom-roles-to-the-new-scheme).

## Getting an equivalent of the basic model's ClusterAdmin and SuperAdmin roles

The granular model has no roles that combine permissions into a single entity, like the `ClusterAdmin` and `SuperAdmin` roles of the [basic model](rbac-current.html). It separates platform administration (system roles) from application access (namespace and project roles). The equivalent is assembled from **two bindings**: a ClusterRoleBinding to a system role and a [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) to a project role (the latter applies in all projects, including those created later).

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
# Platform: DP module configuration, cluster-wide resources, viewing the namespaces of DP modules.
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
Every user namespace is a project: a namespace created directly [becomes a project automatically](/modules/multitenancy-manager/#automatic-project-creation-for-namespaces). Therefore, the "system role + ClusterProjectRoleBinding" pair covers both the platform and all user namespaces. The pair does not cover the `default` namespace: it is a system namespace that belongs to the virtual project `default`, ClusterProjectRoleBinding does not extend to virtual projects, and no DP module runs in it.

You cannot create a custom "all permissions" role (`apiGroups: ["*"], resources: ["*"], verbs: ["*"]`) — it is rejected by the [built-in protection mechanisms](#built-in-protection-mechanisms-of-the-role-model). If you need unrestricted access outside the platform role model, use a ClusterRoleBinding to the built-in Kubernetes `cluster-admin` role — only someone who already has such permissions can assign it.
{% endalert %}
