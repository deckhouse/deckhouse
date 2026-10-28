---
title: "The user-authz module"
description: "Authorization and role-based access control to the resources of the Deckhouse Platform cluster."
---

The module generates role-based access model objects based on the standard Kubernetes RBAC mechanism. The module creates a set of cluster roles (ClusterRole) suitable for most user and group access management tasks.

{% alert level="warning" %}
The module implements two role-based models: the [granular](#granular-role-based-model) one (recommended) and the [basic](#basic-role-based-model) one, built around the ClusterAuthorizationRule and AuthorizationRule resources (its support will be discontinued in future releases).

The models are not resource-compatible — automatic conversion is impossible — but they [can be used at the same time](faq.html#can-the-basic-and-the-granular-role-based-models-be-used-at-the-same-time): the permissions of both models are summed up.
{% endalert %}

The bindings that grant users the access levels of ClusterAuthorizationRule and AuthorizationRule are reconciled by the `user-authz-controller` component: it watches the rules and keeps their ClusterRoleBindings and RoleBindings in sync, and reports the result in the `status` of each rule (`d8 k get clusterauthorizationrules` shows the `READY` and `BINDINGS` columns). A rule with an unsupported `accessLevel` gets `Ready=False` with the `InvalidSpec` reason instead of breaking the module. The controller exports metrics on the rules it sees, the bindings they need, the bindings that exist, and the drift between them. The module ships alerts on controller unavailability, persistent drift, and rules whose bindings are not applied (for details, see the [FAQ](faq.html#how-do-i-check-that-the-controller-keeps-the-bindings-in-sync)).

<div style="height: 0;" id="the-new-role-based-model"></div>
<div style="height: 0;" id="experimental-role-based-model"></div>

## Granular role-based model

Unlike the [basic Deckhouse Platform role-based model](#basic-role-based-model), the granular role-based one does not use ClusterAuthorizationRule and AuthorizationRule resources. Access rights are configured in the standard Kubernetes RBAC way: by creating RoleBinding or ClusterRoleBinding resources and specifying one of the roles prepared by the `user-authz` module in them. To grant access to all namespaces of a project at once, use the [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) and [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) resources of the `multitenancy-manager` module.

{% alert level="info" %}
Access does not have to be granted by hand-writing YAML manifests: the [Deckhouse web UI](/products/kubernetes-platform/documentation/v1/user/web/ui.html) provides an access grant wizard. It walks you through the steps (who gets access → where → at which level), picks the right binding kind itself (RoleBinding, ClusterRoleBinding, ProjectRoleBinding, or ClusterProjectRoleBinding), and lets you assemble a custom role from ready-made building blocks without writing YAML.
{% endalert %}

The module creates special aggregated cluster roles (ClusterRole). By using these roles in RoleBinding or ClusterRoleBinding, you can do the following:

- Manage access to modules of a specific [subsystem](#subsystems-of-the-role-based-model).

  For example, you can use the `d8:subsystem:network:manager` role in ClusterRoleBinding to allow a network administrator to configure *network* modules (such as `cni-cilium`, `ingress-nginx`, `istio`, etc.).
- Manage access to *user* resources of modules within namespaces.

  For example, the `d8:namespace:manager` role in RoleBinding enables deleting/creating/editing the [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) resource in the namespace. At the same time, it does not grant access to the cluster-wide [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) and [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) resources of the `log-shipper` module, nor does it allow configuration of the `log-shipper` module itself.

### Role scopes

Every role operates in one of four scopes. The scope defines *where* the granted permissions apply and *which resource* is used to assign the role:

| Scope | Role name format | Intended for | Which resource is used to assign the role |
|-------|------------------|--------------|-------------------------------------------|
| Namespace | `d8:namespace:<level>` | Application users (developers) | RoleBinding in a specific namespace. |
| Project | `d8:project:<level>` | Teams working with [projects](/modules/multitenancy-manager/) | [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) or [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) for the whole project, RoleBinding for one namespace of the project. |
| Subsystem | `d8:subsystem:<subsystem>:<level>` | Administrators of a part of the platform | ClusterRoleBinding. |
| Whole platform | `d8:system:<level>` | Platform administrators | ClusterRoleBinding. |

The roles created by the module are divided into the following classes:

- [Namespace roles](#namespace-roles) — for assigning rights to users (such as application developers) **in a specific namespace**.
- [Project roles](#project-roles) — for assigning rights **in all namespaces of a project at once**.
- [System and subsystem roles](#system-and-subsystem-roles) — for assigning rights to platform administrators and administrators of a part of the platform.

#### Access levels

Each scope provides several access levels for roles:

- The "namespace" and "project" scopes have five levels: `viewer` → `user` → `manager` → `admin` → `superadmin`.
- The "subsystem" and "whole platform" scopes have three levels: `viewer` → `manager` → `superadmin`. There are no `user` and `admin` levels here: the system level has no "user" resources that could be used without administering them.

Access levels form a hierarchy with a cumulative principle: each next level inherits all permissions of the previous one and extends them. For example, the `manager` level in the namespace scope includes everything the `user` level allows, which in turn includes everything the `viewer` level allows.

### Using ClusterAuthorizationRule and AuthorizationRule together with RBAC

If multitenancy mode is enabled in the cluster (via [`enableMultiTenancy: true`](configuration.html#parameters-enablemultitenancy)), the effective permissions of a user are the combination of all permissions granted by the following sources:

- **Permissions granted by ClusterAuthorizationRule**: Applied only inside the namespaces allowed by its `namespaceSelector` parameter (or the deprecated `limitNamespaces` parameter).
- **Permissions granted by AuthorizationRule**: Applied inside the namespace the AuthorizationRule is created in.
- **Permissions granted by ordinary RoleBinding**: Applied inside the namespace the RoleBinding is created in.
- **Permissions granted by ordinary ClusterRoleBinding** that aren't generated by the `user-authz` module: Applied cluster-wide.

The namespace restrictions set in a ClusterAuthorizationRule apply only to the permissions granted by that ClusterAuthorizationRule. They do not cancel permissions granted via RoleBinding, ClusterRoleBinding, or AuthorizationRule in other namespaces. Likewise, the permissions granted by ClusterAuthorizationRule do not extend to those namespaces.

The namespace restrictions are enforced by the authorization webhook, so they do not apply to the identities that the webhook does not check: control plane components, kubelets, and the ServiceAccounts of `kube-system` and of the namespaces whose names start with `d8-`. A ClusterAuthorizationRule that names such a ServiceAccount, directly or through a group such as `system:serviceaccounts`, grants its access level in every namespace. Create the ServiceAccounts of external systems in ordinary namespaces.

For example, if a user has a ClusterAuthorizationRule with `accessLevel: Editor` limited to the `ns-a` namespace and a RoleBinding with the `view` role in the `ns-b` namespace, they get the `Editor` permissions in the `ns-a` namespace and read-only permissions in the `ns-b` namespace. Permissions granted by the RoleBinding aren't restricted by the ClusterAuthorizationRule, and the `Editor` level set in the ClusterAuthorizationRule doesn't apply to the `ns-b` namespace.

<div style="height: 0;" id="use-roles"></div>

### Namespace roles

{% alert level="warning" %}
A namespace role cannot be assigned through a ClusterRoleBinding. Use a RoleBinding in a namespace, or a ProjectRoleBinding or ClusterProjectRoleBinding for all namespaces of a project.
{% endalert %}

Namespace roles are intended to assign rights to a user **in a specific namespace**. Users refer to, for example, developers who use a cluster configured by an administrator to deploy their applications. Such users don't need to manage Deckhouse Platform modules or a cluster, but they need to be able to, for example, create their Ingress resources, configure application authentication, and collect logs from applications.

The namespace role defines permissions for accessing namespaced resources of modules and standard namespaced resources of Kubernetes (Pod, Deployment, Secret, ConfigMap, etc.).

The module creates the following namespace roles:
- `d8:namespace:viewer` — allows viewing standard Kubernetes resources (except for Secrets and RBAC resources), Gateway API resources, pod logs and metrics in a specific namespace, as well as authenticating in the cluster;
- `d8:namespace:user` — in addition to the role `d8:namespace:viewer` it allows viewing secrets in a specific namespace, connecting to pods (`kubectl exec`, `kubectl attach`), deleting pods (but not creating or modifying them), executing `kubectl port-forward` and `kubectl proxy`, as well as changing the number of replicas of controllers;
- `d8:namespace:manager` — in addition to the role `d8:namespace:user` it allows managing module resources (for example, Certificate, PodLoggingConfig, etc.) and standard namespaced Kubernetes resources (Pod, Deployment, ConfigMap, Secret, Service, Ingress, NetworkPolicy, CronJob, etc.), as well as Gateway API resources (Gateway, ListenerSet, HTTPRoute, GRPCRoute, ReferenceGrant, etc.) in a specific namespace;
- `d8:namespace:admin` — in addition to the role `d8:namespace:manager` it allows managing the resources ResourceQuota, LimitRange, ServiceAccount, Role, RoleBinding in a specific namespace;
- `d8:namespace:superadmin` — in addition to the role `d8:namespace:admin` it allows security-sensitive operations: minting ServiceAccount tokens, making requests on behalf of ServiceAccounts, and managing [system resources placed in the namespace](#admin-level-restrictions-and-superadmin-rights) (for example, Dex pods or pods/PVCs of virtual machines).

The detailed split of permissions between `admin` and `superadmin` is described in the section [Admin level restrictions and superadmin rights](#admin-level-restrictions-and-superadmin-rights).

#### Automatic access to cluster-wide catalog resources

Working in a namespace requires reading some cluster-wide "catalogs": for example, to set `storageClassName` or `ingressClassName` in a manifest, one needs to see the list of StorageClasses and IngressClasses. Therefore, every subject that receives a RoleBinding to any `d8:namespace:*` or `d8:project:*` role automatically also gets read access to such catalog resources (StorageClass, IngressClass, PriorityClass, RuntimeClass, VolumeSnapshotClass, ClusterLogDestination, etc.).

Technically this appears as an automatically created ClusterRoleBinding to the `d8:dict` role labelled `rbac.deckhouse.io/dict: "true"`. These objects are managed by the platform: they appear when the subject gets its first namespace binding and are removed when the subject has none left — no manual editing is needed.

### Project roles

{% alert level="warning" %}
A project role cannot be assigned via ClusterRoleBinding — such an attempt is rejected. To assign a role across a whole project, use [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) or [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). A plain RoleBinding in one of the project namespaces is also allowed — the role then applies in that namespace only.
{% endalert %}

Project roles (`d8:project:<level>`) are intended for working with [projects](/modules/multitenancy-manager/) — isolated environments that may span several namespaces. The levels are the same as for namespace roles: `viewer`, `user`, `manager`, `admin`, `superadmin`.

Each project role includes all permissions of the namespace role of the same level and additionally grants permissions to manage the project itself:

- `d8:project:viewer` — the permissions of `d8:namespace:viewer` plus viewing the project's [ProjectNamespace](/modules/multitenancy-manager/cr.html#projectnamespace) and [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) resources;
- `d8:project:user` — the permissions of `d8:namespace:user` plus viewing the project's ProjectNamespace and ProjectRoleBinding resources;
- `d8:project:manager` — the permissions of `d8:namespace:manager` plus managing the project's additional namespaces (ProjectNamespace resources);
- `d8:project:admin` — the permissions of `d8:namespace:admin` plus managing access to the project (ProjectRoleBinding resources) and the right to bind the built-in `d8:project:*` and `d8:namespace:*` roles (except the `superadmin` level) to other users within the project;
- `d8:project:superadmin` — analogous to the relation between `d8:namespace:superadmin` and `d8:namespace:admin`.

A role assigned via ProjectRoleBinding automatically applies in **all** namespaces of the project — the main one as well as the additional ones, including those created later.

<div style="height: 0;" id="manage-roles"></div>

### System and subsystem roles

{% alert level="warning" %}
The system and subsystem roles do not grant access to the namespace of user applications.

They grant access only to system namespaces (starting with `d8-` or `kube-`), and only to those system namespaces where the modules of the corresponding role subsystem are running.

Whether a role lets you grant access to other people is described in [Who can assign roles](#who-can-assign-roles).
{% endalert %}

System and subsystem roles are intended for assigning rights to manage the entire platform or a part of it (the [subsystem](#subsystems-of-the-role-based-model)), but not the users applications themselves. The subsystem role, for example, can allow a security administrator to manage security modules (responsible for the security functions of the cluster). Thus, the security administrator will be able to configure security policies, certificate issuance, etc., but will not be able to manage other cluster functions (such as access management, network and monitoring settings) or change settings in the namespaces of users applications.

{% alert level="warning" %}
A system/subsystem role limits which modules and namespaces a subject can address, but it does not limit the privileges a subject can obtain through the modules it is allowed to manage. This matters most for the `iam` subsystem, because the right to manage authentication and authorization is equivalent to full control over the cluster.

A subject that can manage the `user-authn` module can register an identity provider or reset the credentials of any local user. A subject that can manage the `user-authz` module can modify authorization rules. In both cases, they can obtain an identity with any privileges, including cluster administrator, so treat an `iam` subsystem role as a cluster administrator role when planning access.
{% endalert %}

The system/subsystem role defines access rights:
- to cluster-wide Kubernetes resources;
- to manage Deckhouse Platform modules (ModuleConfig resource) within the [subsystem](#subsystems-of-the-role-based-model) of the role, or to all Deckhouse Platform modules for the role `d8:system:*`;
- to manage cluster-wide resources of Deckhouse Platform modules within the [subsystem](#subsystems-of-the-role-based-model) of the role, or to all resources of Deckhouse Platform modules for the role `d8:system:*`;
- to read the namespaces of the modules of the role's [subsystem](#subsystems-of-the-role-based-model), or of all Deckhouse Platform modules for the role `d8:system:*`; writing there is reserved for the `superadmin` level (see [Access of system and subsystem roles to the namespaces of the modules](#access-of-system-and-subsystem-roles-to-the-namespaces-of-the-modules)).

The role name format is `d8:system:<ACCESS_LEVEL>` for the system roles and `d8:subsystem:<SUBSYSTEM>:<ACCESS_LEVEL>` for the subsystem roles, where:
- `SUBSYSTEM` is the role's [subsystem](#subsystems-of-the-role-based-model);
- `ACCESS_LEVEL` is the access level.

  Examples:
  - `d8:system:viewer` — access to view the configuration of all Deckhouse Platform modules (ModuleConfig resource), their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except Secrets and RBAC resources) in the namespaces of all Deckhouse Platform modules;
  - `d8:system:manager` — similar to the role `d8:system:viewer`, but with the right to view/create/modify/delete the configuration of all Deckhouse Platform modules (ModuleConfig resource) and their cluster-wide resources; in the namespaces of all Deckhouse Platform modules it keeps the read access of `d8:system:viewer`;
  - `d8:subsystem:observability:viewer` — access to view the configuration of Deckhouse Platform modules (ModuleConfig resource) from the `observability` area, their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except secrets and RBAC resources) in the system namespaces `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

The module provides three access levels for system and subsystem roles:
- `viewer` — allows viewing standard Kubernetes resources, the configuration of modules (ModuleConfig resources), cluster-wide resources of modules, and namespaced resources of modules in the module namespace;
- `manager` — in addition to the level `viewer` it allows managing the configuration of modules (ModuleConfig resources) and the cluster-wide resources of modules; in the module namespaces it grants the same read access as `viewer`;
- `superadmin` — in addition to the level `manager` it allows everything in the module namespaces: managing standard Kubernetes resources and the namespaced resources of modules, reading secrets, connecting to pods, issuing ServiceAccount tokens and managing the system resources of the subsystem modules.

### Access of system and subsystem roles to the namespaces of the modules

The holder of a system or subsystem role automatically gets a RoleBinding to a namespace role in every system namespace in which the modules of the role's subsystem run (a `d8:system:*` role covers all system namespaces): the `viewer` and `manager` levels get `d8:namespace:viewer`, the `superadmin` level gets `d8:namespace:superadmin`. The platform creates and removes these RoleBindings itself (they carry the label `rbac.deckhouse.io/automated: "true"`), one per ClusterRoleBinding of the role.

The namespaces of the modules belong to the platform. They hold ServiceAccounts with cluster-wide rights (the Deckhouse controller among them) and the credentials of the modules, so any write there, and any read of a Secret there, is in effect the right to act as those ServiceAccounts anywhere in the cluster. This is why a `manager` configures its subsystem through ModuleConfig objects and the cluster-wide resources of the modules, and in the module namespaces only looks: objects, logs, metrics and events, but no secrets, no `kubectl exec`, `attach` or `port-forward`, no deleting or scaling of pods and no creating of anything. A `superadmin` can do all of that. Grant this level deliberately and to a named subject: it is the emergency path into the platform, and its actions are visible in the audit log. Do not use impersonation of system users (`--as`) as a substitute.

Keep in mind:

- A RoleBinding to `d8:namespace:user` or a higher level in a system namespace, created by hand, grants the subject the rights of every ServiceAccount of that namespace. The same holds for the `allowAccessToSystemNamespaces: true` option of a ClusterAuthorizationRule of the previous role model: with it, the `Admin` and `ClusterAdmin` access levels are equivalent to a cluster administrator.
- The system namespaces are those the multitenancy-manager module never turns into projects: `d8-*`, `kube-*`, `upmeter-*`, `default` and the namespaces labelled `heritage: deckhouse` or `heritage: upmeter`.
- A custom system or subsystem role (`d8:custom:*`) is projected as `viewer` whatever its `rbac.deckhouse.io/use-role` label says: the webhook refuses a higher value, and a role created with one earlier is projected as `viewer` with a `ProjectionCapped` event on the role.
- For module authors: the system capability of a module (the one aggregated into the `manager` role of its subsystem) must not grant write access to namespaced objects of the module's namespace that produce workloads with a ServiceAccount, tokens or identities (pods and pod controllers, Secrets, ServiceAccounts, Roles and RoleBindings): such a right would hand the `manager` the ServiceAccounts of the module. The `dmt` linter will check this for the `rbac.yaml` of a module.

### Who can assign roles

A system or subsystem role does not, by itself, let you grant access to other people.

Creating a User or Group that is not already a grant subject is ordinary object creation.

Creating a User for an email that already carries a grant, writing or deleting a ClusterAuthorizationRule, or resetting that identity through UserOperation, is granting roles. The request is admitted only if the requester already has covering permissions or is explicitly allowed to assign those roles.

Connecting a DexProvider is measured the same way against the identities the provider can assert: [`spec.allowedIdentities`](/modules/user-authn/cr.html#dexprovider-v1-spec-allowedidentities) (emails, email domains, groups) and the provider's own group filters bound that set, and only the roles already granted inside it count. A provider without an email limit can assert every email, including the ones that hold SuperAdmin, so writing it requires SuperAdmin. Basic Authentication of an OIDC or a Crowd provider bypasses Dex and its limits, so such a provider counts as one without an email limit, and an OIDC one also without a group limit; a Crowd provider keeps only `crowd.groups` on the group axis. Details and the list of changes admitted without a check are in the user-authn module documentation.

The ClusterAuthorizationRule `spec.accessLevel` field is a [basic-model](#basic-role-based-model) level: `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, `ClusterAdmin`, `SuperAdmin`. An `iam` subsystem manager and a holder of `d8:system:manager` assign the levels `ClusterEditor` includes: `User`, `PrivilegedUser`, `Editor` and `ClusterEditor`, but not `Admin`, `ClusterAdmin` or `SuperAdmin`, and neither the Kubernetes `cluster-admin` ClusterRole.

A ClusterAuthorizationRule that opens the system namespaces counts as a `ClusterAdmin` assignment whatever its `accessLevel`, so these managers cannot write it: a rule with `allowAccessToSystemNamespaces: true`, with `namespaceSelector.matchAny`, with a `namespaceSelector.labelSelector` that does not name its namespaces by the `kubernetes.io/metadata.name` label, a rule for a subject that another rule already lets into them (the rules of a subject add up), or any rule while multitenancy mode is disabled. Without multitenancy mode, as in CE, these managers therefore cannot write ClusterAuthorizationRule objects at all.

The rules of a request add up: those of its user, of every group in its token and of its ServiceAccount. A rule therefore also opens the system namespaces when another rule already lets in one of the requests it applies to: a local user it names or a member of its groups at any depth of nesting, with the groups that user is in and the groups above them; the requester, with the groups of its own token; any request, through `system:authenticated` (a ServiceAccount also through `system:serviceaccounts` and `system:serviceaccounts:<namespace>`).

A rule for `system:authenticated` applies to every request, so it opens the system namespaces as soon as any other rule opens them. A rule for `system:serviceaccounts` does so as soon as another rule lets in a ServiceAccount, and a rule for `system:serviceaccounts:<namespace>` as soon as another rule lets in a ServiceAccount of that namespace. A User subject named `system:serviceaccount:<namespace>:<name>` is that ServiceAccount.

A member of a Group gets the rules of every group above it, so writing a Group needs the range for their roles too. For the same reason, these managers cannot add a member that another rule already lets into the system namespaces to a Group that has a ClusterAuthorizationRule, or is in a group that has one. The groups an identity provider asserts for other users are unknown to the webhook and are not taken into account.

For the same reason a user or a group that such a rule names counts as a `ClusterAdmin` target: these managers cannot create a User for its email, add members to such a Group, run a UserOperation on such a user or connect a DexProvider that can assert such an identity. Turning multitenancy mode off in the `user-authz` ModuleConfig opens the system namespaces to every rule and requires the `ClusterAdmin` range as well. Within the granular model the IAM manager assigns the `iam` subsystem roles up to `d8:subsystem:iam:manager`, and the system manager assigns the roles of any scope up to the `manager` level.

These managers do not write admission webhooks and policies (ValidatingWebhookConfiguration, MutatingWebhookConfiguration, ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding): the superadmins of the `security` and `cluster` subsystems and `d8:system:superadmin` do. A mutating webhook can change any object the platform creates, its own pods included, and removing a webhook or a policy switches off what it enforces, such as the limits above. The ValidatingWebhookConfigurations `d8-deckhouse-validating-webhook-handler-*`, which hold the webhooks of the Deckhouse modules, can be created, changed or deleted only by webhook-handler, the platform components and the superadmins, including the holders of a role that grants every verb on everything.

A requester with every permission across the cluster counts as SuperAdmin in these checks and can assign any role, including `SuperAdmin` and `cluster-admin`. For example, a subject of the Kubernetes `cluster-admin` ClusterRole has such permissions if the role is granted by a ClusterRoleBinding or by a ClusterAuthorizationRule without `namespaceSelector` and `limitNamespaces`. If multitenancy mode is enabled, such a rule reaches the system namespaces only with `namespaceSelector.matchAny: true` (or the deprecated `allowAccessToSystemNamespaces: true`).
Permissions from a ClusterAuthorizationRule with `namespaceSelector` or `limitNamespaces` do not count toward this, even if the rule grants `cluster-admin`. Any non-empty `limitNamespaces` counts as a limit on purpose, even if its entries match every namespace, such as `.*` together with `allowAccessToSystemNamespaces: true`. Such a rule still counts through its `accessLevel`. For example, `accessLevel: SuperAdmin` gives the SuperAdmin range whatever the namespace limits of the rule.

`limitNamespaces` and `namespaceSelector` restrict access to namespaced resources only. A subject with the `ClusterAdmin` or `SuperAdmin` access level can still create ClusterRoleBinding and ClusterAuthorizationRule objects, so these fields do not confine what it can grant across the cluster. Give these levels only to subjects that may hold access to the whole cluster. A ClusterAuthorizationRule counts only through the bindings that `user-authz-controller` has created for it.

Webhook deny messages call the `accessLevel` values basic, so they are not confused with the granular role levels (`viewer` / `manager` / `superadmin`).

Permission to create User and Group objects in the `user-authn` module is not enough: it does not grant the roles already attached to that email.

### Global resource dictionaries

In addition to the roles and capabilities of the granular model, the module creates a special ClusterRole `d8:dict`. It
grants **read-only** access to cluster-scoped "reference" resources that users commonly need to discover
when creating objects — for example, a user creating a PersistentVolumeClaim needs to see the available
StorageClasses, and a user creating an Ingress needs to see the IngressClasses.

The role grants `get`, `list`, `watch` on the following resources:

- `storageclasses` (storage.k8s.io), plus `csidrivers`, `csinodes`, `volumeattachments`
- `volumesnapshotclasses` (snapshot.storage.k8s.io)
- `ingressclasses` (networking.k8s.io)
- `priorityclasses` (scheduling.k8s.io)
- `runtimeclasses` (node.k8s.io)
- `virtualmachineclasses`, `clustervirtualimages` (virtualization.deckhouse.io)
- `clusterlogdestinations` (deckhouse.io)
- `customresourcedefinitions` (apiextensions.k8s.io) — `get`, `list` only

#### Automatic binding

`user-authz-controller` keeps one ClusterRoleBinding to `d8:dict` for every subject that holds one of the following RoleBindings:

- a RoleBinding to a `d8:namespace:*` or `d8:project:*` role, including the RoleBindings that `multitenancy-manager` creates from a ProjectRoleBinding or a ClusterProjectRoleBinding;
- a RoleBinding that the module creates for an AuthorizationRule (the `User`, `PrivilegedUser`, `Editor`, and `Admin` levels).

The `system:multitenancy-manager:*` users, which `multitenancy-manager` acts as, get none. This means that project users can discover reference resources without any manual configuration. The binding is removed when the subject has no such RoleBinding left. You do not create it yourself.

{% alert level="info" %}
The `d8:dict` role is independent of the `multitenancy-manager` cluster-wide resource access management mechanism: it only gives
**read** access to reference resources so users can discover them. The mechanism controls *which resource values* a project may actually reference when creating objects.
{% endalert %}

### Subsystems of the role-based model

Each Deckhouse Platform module belongs to a specific subsystem. For each subsystem, there is a set of roles with different levels of access. Roles are updated automatically when the module is enabled or disabled.

The subsystems of the platform are `iam`, `security`, `cluster`, `delivery`, `network`, `storage`, `observability` and `managed-services`.

For example, for the `network` subsystem, there are the following subsystem roles that can be used in `ClusterRoleBinding`:

- `d8:subsystem:network:viewer`;
- `d8:subsystem:network:manager`;
- `d8:subsystem:network:superadmin`.

The scope of a role depends on which subsystem it belongs to:

- The scope of the `d8:system:*` roles is the namespaces of all enabled Deckhouse Platform modules, that is, the namespaces that their capabilities name in the `rbac.deckhouse.io/namespace` label.
- The scope of subsystem roles includes the namespaces in which the subsystem’s modules operate (see the subsystem composition table), as well as all cluster-wide objects of the subsystem’s modules.

Role-based model subsystems composition table.

{% include rbac/rbac-subsystems-list.liquid %}

### How the roles are built: aggregation and capabilities

No built-in role contains a list of permissions directly. Permissions are described in separate small cluster roles — **capabilities**. Each capability is responsible for one kind of action (for example, "view logs", "manage quotas", "connect to pods") and contains concrete RBAC rules. A role (`d8:namespace:admin`, `d8:system:viewer`, etc.) is an empty ClusterRole with an aggregation rule (`aggregationRule`): Kubernetes automatically collects into it the rules from all capabilities with matching labels.

Membership of objects in the role model is defined by the `rbac.deckhouse.io/*` labels — for example, the label `rbac.deckhouse.io/aggregate-to-namespace-as: admin` includes a capability into the `d8:namespace:admin` role (and, thanks to the level hierarchy, into all levels above). The complete list of labels and annotations is in [the reference below](#reference-of-role-labels-and-annotations).

This design has two practical consequences:

- Deckhouse Platform modules extend the roles automatically: when a module is enabled, its capabilities are added to the corresponding built-in roles; when it is disabled, they are removed. The permission list of a role always matches the set of enabled modules.
- You can assemble your own roles from ready-made capabilities without writing RBAC rules by hand. How to do this is described in [the FAQ](faq.html#how-do-i-extend-a-role-or-create-a-new-one).

The names of built-in roles and capabilities start with the `d8:` prefix. This namespace is reserved: you cannot create your own ClusterRole with a `d8:*` name — the only exception is the `d8:custom:*` prefix, which is dedicated to user-defined roles and capabilities. The labels `rbac.deckhouse.io/kind: role` and `rbac.deckhouse.io/kind: capability` are also reserved for built-in objects — use `custom-role` and `custom-capability` for your own.

### Reference of role labels and annotations

Labels the role model uses on ClusterRole objects:

| Label | Found on | Purpose |
|-------|----------|---------|
| `rbac.deckhouse.io/kind` | All role model objects | Object type: `role` or `capability` — built-in (reserved), `custom-role` or `custom-capability` — user-defined. An object without this label is refused if it carries the aggregation label of a platform scope. |
| `rbac.deckhouse.io/scope` | Roles and capabilities | Scope: `namespace`, `project`, `subsystem`, `system`. |
| `rbac.deckhouse.io/subsystem` | Subsystem objects | The subsystem name (for example, `network`) — only with `scope: subsystem`. |
| `rbac.deckhouse.io/aggregate-to-<scope>-as: <level>` | Capabilities and lower-level roles | The aggregation rule: includes the object into the role of the given scope and level. `<scope>` is `system`, `namespace`, `project`, or a subsystem name; `<level>` is `viewer`, `user`, `manager`, `admin`, `superadmin`. These are exactly the labels referenced by `aggregationRule` selectors. |
| `rbac.deckhouse.io/capability` | Capabilities | The globally unique name of the capability (for example, `namespace-capability.kubernetes.view_logs`) — used to include the capability into a [custom role](faq.html#creating-a-custom-namespace-or-project-role) selectively. |
| `rbac.deckhouse.io/use-role: <level>` | System and subsystem roles | Which namespace-role level the holder of this role automatically gets in the system namespaces of its subsystem. On the built-in roles: `viewer` → `viewer`, `manager` → `viewer`, `superadmin` → `superadmin`; a role that is not built into the platform gets `viewer` at most. The access is granted by automatically created RoleBinding objects (labelled `rbac.deckhouse.io/automated: "true"`). |
| `rbac.deckhouse.io/namespace: <namespace>` | Capabilities | The namespace of a module where a RoleBinding is automatically created for the holders of a system/subsystem role. Read only on the capabilities Deckhouse ships; on a custom capability it is ignored ([how to give access to another namespace](faq.html#extending-subsystem-roles-and-adding-a-new-namespace)). |
| `rbac.deckhouse.io/delegatable: "true"` | The `d8:namespace:*`, `d8:project:*`, and user-defined roles | The role may be referenced by a RoleBinding [inside project namespaces](/modules/multitenancy-manager/usage.html#roles-available-in-a-rolebinding-inside-a-project). Add this label to your custom roles that should be available in projects. |
| `rbac.deckhouse.io/deprecated: "true"` | [Deprecated alias roles](#deprecated-role-names) | The role is deprecated and will be removed; migrate the bindings to the new role. |
| `module` | Built-in objects | The name of the Deckhouse Platform module the object belongs to. Convenient for aggregation selectors (for example, all capabilities of one module). |
| `heritage: deckhouse` | Built-in objects | Marks a platform object. Must not be set on your own objects. |

Annotations on ClusterRole objects:

| Annotation | Added by | Purpose |
|------------|----------|---------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | Platform | The [display title and description](#display-names-of-roles) in Russian. |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | Platform | The display title and description in English. |
| `custom.meta.deckhouse.io/title`, `custom.meta.deckhouse.io/description` | Administrator | Overrides the display title/description; the only allowed modification of built-in roles. |
| `rbac.deckhouse.io/bindable-only-via` | Platform | The binding kinds the role is intended for (on project roles — `ProjectRoleBinding,ClusterProjectRoleBinding`). It is informational: the platform does not check it. |
| `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` | Administrator | Forbids granting the role in projects: existing bindings keep working, new ones cannot be created ([details](/modules/multitenancy-manager/usage.html#granting-access-within-a-project)). |
| `rbac.deckhouse.io/deprecated-replaced-by` | Platform | On deprecated alias roles: the name of the role to migrate to. |

### Admin level restrictions and superadmin rights

The role model provides two administration levels:

- `admin` — the everyday administrator. Manages resources, quotas, and access within their scope, but cannot perform operations that would let them break out of that scope or disrupt platform components.
- `superadmin` — the "break-glass" administrator. Has all the rights of `admin` and can additionally perform dangerous operations. Grant this level deliberately and only to those who really need it.

What is forbidden at the `admin` level and allowed only at the `superadmin` level:

- **Minting ServiceAccount tokens** (`kubectl create token`) **and making requests on behalf of a ServiceAccount** (`kubectl --as system:serviceaccount:...`). A ServiceAccount token is a ready-to-use identity: by obtaining the token of a platform component's service account, one could gain its permissions far beyond the namespace. Therefore `admin` manages the `ServiceAccount` objects themselves (create, delete) but cannot mint tokens for them or act on their behalf.
- **Modifying and deleting system resources in user namespaces.** Some platform components place their objects (for example, Dex authenticator pods, or pods and disks of virtual machines) directly in application namespaces. Such objects carry the `deckhouse.io/system-resource: "true"` label. Only `superadmin` may modify or delete them; for everyone else these operations are rejected at the API server level with an explanation.
- **Connecting to system pods** — `kubectl exec`, `kubectl attach`, and `kubectl port-forward` into a pod labelled `deckhouse.io/system-resource: "true"` are available to `superadmin` only. This protects against reading foreign secrets and interfering with platform components from inside their pods.

Module authors: how to mark the objects that a module places in user namespaces is described in the [FAQ](faq.html#how-do-i-protect-objects-that-my-module-places-in-user-namespaces).

The restrictions above do not apply to cluster administrators. A cluster administrator is anyone granted the standard Kubernetes `cluster-admin` role or the `user-authz:super-admin` role of the previous role model (which the subjects of a ClusterAuthorizationRule with the `SuperAdmin` access level receive), as well as members of the `system:masters`, `kubeadm:cluster-admins`, `superadmins`, and `system:sudousers` groups. How the role was granted does not matter: both a ClusterRoleBinding and a RoleBinding in the namespace in question work. Such users already hold full rights over the cluster, so they keep their break-glass access.

`superadmin` also has some restrictions:

- Resources created from a [project template](/modules/multitenancy-manager/) (the `heritage: multitenancy-manager` label) cannot be modified by **anyone**, including `superadmin` and a cluster administrator — they are managed exclusively by the project controller. To change such a resource, change the project template or the project itself.
- A role is assigned via RoleBinding and applies only in the namespace where it was granted: a `superadmin` of one namespace gets no special rights in another.

### Built-in protection mechanisms of the role model

The role model is protected by a set of checks at the API server level. They require no configuration and prevent typical mistakes and privilege escalation attempts:

- **A scoped role cannot be granted cluster-wide.** A ClusterRoleBinding to the `d8:namespace:*` or `d8:project:*` roles (and their `d8:custom:*` variants) is rejected — otherwise a role designed for one namespace or project would apply in every namespace at once. Use a RoleBinding in the desired namespace, or ProjectRoleBinding/ClusterProjectRoleBinding for a project. A ClusterRoleBinding is only allowed for system and subsystem roles — those are cluster-scoped by nature.
- **A capability cannot be granted cluster-wide.** A ClusterRoleBinding to any capability (`d8:*-capability:*`, including custom ones) is rejected: a capability is a building block for roles, not a standalone role. Binding a capability via a RoleBinding in a single namespace is allowed.
- **Project management cannot be obtained through a custom role.** Creating a Role or ClusterRole that grants permissions to modify project management resources (`projects`, `projecttemplates`, `projectrolebindings`, `clusterprojectrolebindings`, `projectnamespaces`) is rejected. These permissions are granted only by the built-in `d8:project:*` roles. Without this protection, a namespace administrator could create a role with the right to create ProjectRoleBinding objects and grant themselves access to the whole project. Read-only roles on these resources are allowed.
- **A namespace or project role cannot include platform capabilities.** A custom role with `scope: namespace`, `scope: project`, or no scope may not select system or subsystem capabilities by the `rbac.deckhouse.io/capability` label, nor the `system` or a subsystem scope by the `rbac.deckhouse.io/aggregate-to-<scope>-as` label. No custom role may combine the capabilities of the `namespace`/`project` scopes with those of the `system`/`subsystem` scopes — this rules out a "super-role" combining access to applications and to the platform.
- **A custom capability carries rules and cannot have an `aggregationRule`.**
- **The name must match the scope**: `d8:custom:namespace:<name>`, `d8:custom:project:<name>`, `d8:custom:system:<name>`, `d8:custom:<subsystem>:<name>`, and `d8:custom:<scope>-capability:<name>` for a capability.
- **Only a custom capability may carry an `rbac.deckhouse.io/aggregate-to-<scope>-as` label of a platform scope.**
- **The `rbac.deckhouse.io/delegatable: "true"` label is accepted only on a role that aggregates only namespace and project capabilities.**
- **Custom roles cannot contain direct RBAC rules** — they may only aggregate capabilities. Permissions are described in separate custom capabilities, which keeps the contents of any role transparent. See [the FAQ](faq.html#how-do-i-extend-a-role-or-create-a-new-one) for details.

### Display names of roles

Every built-in role and capability carries a localized title and description in annotations:

- `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` — in Russian;
- `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` — in English.

These annotations are used, for example, by the Deckhouse web UI when displaying the list of roles.

If the standard title does not fit (for example, you want to name roles in your company's terms), add the `custom.meta.deckhouse.io/title` and `custom.meta.deckhouse.io/description` annotations to the role — the interface will show them instead of the standard ones. Example:

```shell
d8 k annotate clusterrole d8:namespace:admin custom.meta.deckhouse.io/title='Team administrator'
```

This is the only allowed modification of built-in roles: changing their rules, aggregation, or labels is rejected.

<div style="height: 0;" id="deprecated-role-names"></div>

### Deprecated role names

The previous role names of the granular model (`d8:manage:<subsystem>:<level>`, `d8:manage:all:<level>`, and `d8:use:role:<level>`) are deprecated and will be removed in the next release. For backward compatibility they are temporarily kept as alias roles. A binding to an alias aggregates the **new** role's capabilities, except where the list of subsystems changed (see [below](#the-subsystem-list-of-the-aliases)).
The rights of an alias are not identical to the pre-upgrade ones. `d8:use:role:admin` no longer grants ServiceAccount token minting or impersonation, which moved to `superadmin`. The `d8:manage:*` aliases are not `delegatable`. The `d8:use:role:*` aliases are, so a RoleBinding to them inside a project namespace keeps working, but a new ProjectRoleBinding or ClusterProjectRoleBinding to an old name is refused.

A ClusterRoleBinding to a `d8:manage:*` alias keeps the RoleBindings that the previous release created for it in the namespaces of the modules of the old subsystem. For a `viewer` alias they are named `d8:use:viewer:binding:<ClusterRoleBinding name>` and refer to `d8:use:role:viewer`, for a `manager` alias `d8:use:user:binding:<ClusterRoleBinding name>` and `d8:use:role:user`. They bind the subjects of the ClusterRoleBinding and are kept in sync as before.
The namespaces are those of the platform modules that were in the old subsystem and of the capabilities of modules that still carry the old labels, `rbac.deckhouse.io/kind: manage` together with the aggregation label of the old subsystem. For `d8:manage:all:<level>` they also include the namespaces that the subsystem roles of the modules with a subsystem of their own reach at its level, for example `d8-virtualization`. A binding to an alias therefore gets no RoleBinding in the namespace of a module that joined the subsystem later or of a module released on its own that already carries the new labels. A ClusterRole created by a user adds no namespace, whatever labels it carries.
The RoleBindings that the previous release created for the binding stay while the binding refers to the old name, even in a namespace that is no longer among these, for example when a module released on its own moves to the new labels. A ClusterRoleBinding that is deleted and created again under the same name keeps them only in these namespaces.
When you recreate the binding on a new role name, these RoleBindings are replaced by `d8:namespace:<level>:binding:<ClusterRoleBinding name>` ones in the namespaces of the modules of the new subsystem, with the namespace role of the new role (`admin` for a `manager` role).
The RoleBindings that the previous release created for a binding to a custom role of the previous scheme with `rbac.deckhouse.io/kind: manage`, such as `custom:manage:mycustom:manager`, are removed on the upgrade, because `user-authz-controller` keeps such RoleBindings for the roles of Deckhouse only. They come back as `d8:namespace:<level>:binding:<ClusterRoleBinding name>` once the role is recreated as a `d8:custom:*` role with `rbac.deckhouse.io/scope: system` or `subsystem` and `rbac.deckhouse.io/use-role` and the binding refers to the new role, but only in the namespaces of the modules that already carry the new labels. A module that still carries the old labels gets no such RoleBinding until it moves to the new ones.
Until then, create RoleBinding objects in the namespace of that module for the subjects of the binding: one to `d8:namespace:<level>` and one to each ClusterRole of the module that the `D8UserAuthzLegacyUseCapability` alert names. None of these roles is an alias, so the RoleBinding objects do not hold the next release, and the webhook that checks RoleBinding objects in project namespaces does not check the namespace of a module. Delete them once the module carries the new labels and the controller creates its own RoleBinding there.
A RoleBinding to `d8:use:role:<level>` gives these rights as well, but the `D8UserAuthzDeprecatedRBACv2RoleInUse` alert reports it, and it holds the release that removes the aliases.

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

The list of subsystems changed together with the role names. `deckhouse`, `infrastructure` and `kubernetes` were merged into `cluster`, which also took over modules from other subsystems, `networking` was renamed to `network`, and `user-authz`, `user-authn` and `multitenancy-manager` moved to `iam`.

The aliases of the `deckhouse`, `infrastructure` and `kubernetes` roles therefore do not aggregate the whole `cluster` subsystem. They select the capabilities of the platform modules that were in the old subsystem, wherever those modules are now. `d8:subsystem:cluster:<level>` grants more, so when you recreate such a binding, check which subsystem roles its subjects need.
The alias of `d8:manage:security:<level>` also keeps the rights for `user-authz` and `user-authn`, which now come with `d8:subsystem:iam:<level>`. That role also grants the rights for `multitenancy-manager` and, at the `manager` level, the right to assign roles, which `d8:subsystem:security:<level>` does not grant.
So a binding to `d8:manage:security:<level>` is replaced by two bindings with the same subjects, to `d8:subsystem:iam:<level>` and to `d8:subsystem:security:<level>`. The `rbac.deckhouse.io/deprecated-replaced-by` annotation of the alias names only the second one.
The aliases of `networking` and `storage` leave out `keepalived` and `local-path-provisioner`, which moved to `network` and `storage` from `infrastructure`, so the new roles grant more than these aliases.

The `rbac.deckhouse.io/deprecated-replaced-by` annotation of an alias names its main replacement, and the description of the alias names the roles of the other subsystems its modules moved to. These roles cover whole subsystems. To grant the rights for some modules only, create a custom role that selects their system capabilities by the `rbac.deckhouse.io/capability` label, for example `system-capability.multitenancy-manager.view` (see [How do I extend a role or create a new one?](faq.html#how-do-i-extend-a-role-or-create-a-new-one)).

The aliases of the `deckhouse`, `infrastructure`, `kubernetes` and `networking` roles and `d8:manage:all:<level>` also select the labels of these old subsystems, such as `rbac.deckhouse.io/aggregate-to-deckhouse-as`. Modules released on their own and custom capabilities may still carry them. No role of the new model collects these labels, so such capabilities reach the holders of the old names only, and the `D8UserAuthzRetiredAggregationLabel` alert names them. A module released on its own that has moved to its new subsystem is no longer selected by the alias of its old one.
`d8:manage:all:<level>` also selects `rbac.deckhouse.io/kind: manage` together with `rbac.deckhouse.io/aggregate-to-all-as` at its level, as the previous release did. Modules with a subsystem of their own carry this label on their subsystem roles, for example `d8:manage:virtualization:<level>`, until they move to the new labels.

The `d8:use:role:<level>` and `d8:use:role:<level>:kubernetes` aliases also select the use capabilities of the previous release, `rbac.deckhouse.io/kind: use` together with `rbac.deckhouse.io/aggregate-to-kubernetes-as` at the level of the alias or below. Modules released on their own and custom capabilities may still carry them, for example the `log-shipper` capabilities for PodLoggingConfig objects.
`d8:use:role:<level>` also selects `rbac.deckhouse.io/kind: use` together with `rbac.deckhouse.io/aggregate-to-all-as` at its own level, as the previous release did. Modules with a subsystem of their own carry this label on their use roles, for example `d8:use:role:<level>:virtualization`.
`d8:namespace:<level>` does not select them, so a RoleBinding to an old name keeps these rights, and a RoleBinding to the new name gets them only once the module or the capability carries `rbac.deckhouse.io/aggregate-to-namespace-as`. The `D8UserAuthzLegacyUseCapability` alert names the use capabilities with `rbac.deckhouse.io/aggregate-to-kubernetes-as` and the use roles with `rbac.deckhouse.io/aggregate-to-all-as`.
The alert also names a ClusterRole with `rbac.deckhouse.io/kind: use` and the aggregation label of the system scope, of a built-in subsystem, or of `deckhouse`, `infrastructure` or `networking`. No role collects such a ClusterRole, so only a binding to the ClusterRole itself gives its rules.
The `rbacv2-cluster-roles.deckhouse.io` webhook does not let a user create such a ClusterRole, and lets a user change one left from the previous release only by removing the aggregation label from it.

Until a module carries the new labels, create a RoleBinding to the ClusterRole that `D8UserAuthzLegacyUseCapability` names, such as `d8:use:role:<level>:virtualization` or the use capability itself, for the subjects that need these rights, in the namespace where they have their RoleBinding to `d8:namespace:<level>`. Neither role is an alias, so these RoleBinding objects do not hold the next release.
In a project namespace, the `cluster-objects-grants-validator.multitenancy.deckhouse.io` webhook admits such a RoleBinding only if the ClusterRole has the `rbac.deckhouse.io/delegatable: "true"` label, and the use capabilities of the previous release do not have it.
Where the webhook refuses it, keep the RoleBinding on the old name. It keeps these rights, but it holds the release that removes the aliases, so update the module before that release and then move the RoleBinding to `d8:namespace:<level>`.

Within the modules it covers, a binding to an old name gets their capabilities as they are in this release. These differ from the previous release, because the modules added resources to them. For example, the `multitenancy-manager` capabilities include the ProjectRoleBinding, ClusterProjectRoleBinding and ProjectNamespace objects, and the `iam` and `security` managers can inspect the access of any subject.

{% alert level="warning" %}
The deprecated `d8:use:role:admin` role maps to `d8:namespace:admin` and, just like it, [no longer grants](#admin-level-restrictions-and-superadmin-rights) the right to mint ServiceAccount tokens or impersonate ServiceAccounts — that now requires the `superadmin` level.
{% endalert %}

As long as bindings to the deprecated names remain in the cluster, the `D8UserAuthzDeprecatedRBACv2RoleInUse` (a binding to an alias role) and `D8UserAuthzDeprecatedRBACv2CapabilityInUse` (a binding to a deprecated name that no ClusterRole has, such as a capability of the previous release, so the binding grants nothing) alerts fire. The release that removes the aliases carries the `deprecatedRBACv2BindingsCount` release requirement: the upgrade stays pending, with the offending bindings listed in the `DeckhouseRelease` status, until every binding to an alias role is recreated on a new role name. A binding that grants nothing does not hold the release: removing the aliases changes nothing for it.
An alias role here is a ClusterRole of the `user-authz` module with the `rbac.deckhouse.io/deprecated` label. A module released on its own may still ship roles and capabilities of the previous release under the deprecated names, such as `d8:manage:virtualization:<level>`, `d8:use:role:<level>:virtualization` or `d8:manage:permission:module:ingress-nginx:edit`. A binding to such a ClusterRole keeps working, raises neither alert and does not hold the release.
The alerts and the release requirement leave out the RoleBindings that `user-authz-controller` keeps for a ClusterRoleBinding to an alias, because these follow the ClusterRoleBinding.

A module that renames a role of its own keeps the old name the same way: a ClusterRole labelled `rbac.deckhouse.io/deprecated` in the module's `templates/rbacv2-compat/`, with the new name in the `rbac.deckhouse.io/deprecated-replaced-by` annotation. A binding to such an alias raises `D8UserAuthzDeprecatedRBACv2ModuleRoleInUse`, which names the new role, and does not hold the Deckhouse release: the module removes the alias in a release of its own. The annotation is required (an alias without it is ignored), and the alias may have any `d8:` name except the platform's own role families (`d8:system:`, `d8:subsystem:`, `d8:namespace:`, `d8:project:`, `d8:custom:`, `d8:dict`), the reserved prefixes `d8:binding:` and `d8:prb:`, and the previous platform names `d8:manage:` and `d8:use:`.

Migrate your existing RoleBinding and ClusterRoleBinding objects to the new role names.
A ClusterRoleBinding named `user-authz:<rule>:additional-role:<role>` is rendered by `user-authz-controller` from `spec.additionalRoles` of the ClusterAuthorizationRule `<rule>`, and the controller creates it again when it is deleted. So change the role name in `spec.additionalRoles` of that rule, not the binding.
A binding that a project template renders is changed in the ProjectTemplate the same way, because `multitenancy-manager` renders it again from the template.
You can find bindings that still use the deprecated names, without the RoleBindings that `user-authz-controller` keeps for them, with the command:

```shell
d8 k get clusterrolebindings,rolebindings -A -o json \
  | jq -r '.items[] | select(.roleRef.name | test("^d8:(manage|use):")) | select(.metadata.labels["rbac.deckhouse.io/automated"] != "true") | "\(.kind) \(.metadata.namespace // "-") \(.metadata.name) -> \(.roleRef.name)"'
```

<div style="height: 0;" id="the-obsolete-role-based-model"></div>
<div style="height: 0;" id="current-role-based-model"></div>

## Basic role-based model

Features:
- Access is configured with the ClusterAuthorizationRule and AuthorizationRule [custom resources](cr.html), on top of Kubernetes RBAC;
- Manages access to scaling tools (the `allowScale` parameter of the [ClusterAuthorizationRule](cr.html#clusterauthorizationrule-v1-spec-allowscale) or [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-allowscale) custom resource);
- Manages access to port forwarding (the `portForwarding` parameter of the [ClusterAuthorizationRule](cr.html#clusterauthorizationrule-v1-spec-portforwarding) or [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-portforwarding) custom resource);
- Manages the list of allowed namespaces with a labelSelector (the `namespaceSelector` parameter of the [ClusterAuthorizationRule](cr.html#clusterauthorizationrule-v1-spec-namespaceselector) custom resource);
- Namespace restrictions (multi-tenancy mode, `namespaceSelector`, `limitNamespaces`) are available in the BE, SE, SE+, EE, and CSE editions.

In addition to the RBAC, you can use a set of high-level roles in the module:
- `User` — has access to information about all objects (including viewing pod logs) but cannot exec into containers, read secrets, and perform port-forwarding;
- `PrivilegedUser` — the same as `User` + can exec into containers, read secrets, and delete pods (and thus, restart them);
- `Editor` — the same as `PrivilegedUser` + can create, edit, and delete the objects that are usually required for application tasks;
- `Admin` — the same as `Editor` + can create and modify pods, create ServiceAccount tokens (`serviceaccounts/token`), manage Role, RoleBinding, and AuthorizationRule objects, and delete auxiliary objects (for example, ReplicaSet and the challenges and orders of the `acme.cert-manager.io` group);
- `ClusterEditor` — the same as `Editor` + can manage a limited set of cluster-wide objects used in application tasks (DaemonSet, StorageClass, CustomResourceDefinition, GatewayClass, and the objects that modules add at this level, for example, KeepalivedInstance). This role is best suited for cluster operators.
- `ClusterAdmin` — the same as both `ClusterEditor` and `Admin` + can manage cluster-wide service objects (namespaces, quotas, NetworkPolicy, the proxy subresources of nodes, ClusterAuthorizationRule, ClusterRole, and ClusterRoleBinding). This role is best suited for cluster administrators. **Warning.** `ClusterAdmin` can create ClusterAuthorizationRule objects only up to the `ClusterAdmin` level, and ClusterRoleBinding objects only to roles whose permissions it already holds;
- `SuperAdmin` — can perform any actions with any objects (note that `namespaceSelector` and `limitNamespaces` restrictions remain valid).

{% alert level="warning" %}
Multi-tenancy mode restricts access to namespaced resources only. Subjects with the `ClusterAdmin` or `SuperAdmin` access level can still create cluster-wide bindings. The authorization webhook does not check control plane components, kubelets, and the ServiceAccounts of `kube-system` and of the namespaces whose names start with `d8-`, so the namespace restrictions do not apply to them.
{% endalert %}

If a [ClusterAuthorizationRule](cr.html#clusterauthorizationrule) custom resource contains the `namespaceSelector` field, the deprecated `limitNamespaces` and `allowAccessToSystemNamespaces` fields are not taken into consideration.

In multi-tenancy mode, if the authorization webhook becomes unavailable, requests are **denied**, not allowed: the webhook runs before RBAC and is configured with `failurePolicy: Deny`, so a request it cannot answer does not reach RBAC. The exception is the identities listed in `matchConditions` of the AuthorizationConfiguration: control plane components, `kubernetes-api-proxy`, kubelets, the ServiceAccounts of the `kube-system` and `d8-*` namespaces, and anonymous kube-apiserver health probes (`/livez`, `/readyz`, `/healthz`) bypass the webhook, so the cluster keeps running and Deckhouse can restore the webhook.

In multi-tenancy mode on Kubernetes 1.34 or later, the API server also asks the authorization webhook whether the `user-authn` module has revoked the ID token of a user. The module revokes tokens on a `Lock`, `ResetPassword` or `Reset2FA` [UserOperation](/modules/user-authn/cr.html#useroperation), on a `Logout` UserOperation without the `logout` field, and when a User is deleted. The API server denies a revoked token within seconds with the `user-authn: the credentials of this user were revoked; sign in again` message. This check does not deny requests when the webhook is unavailable or the `user-authn` module is disabled; a revoked ID token then stays valid until it expires. The check does not apply to Basic Authentication, ServiceAccount tokens, client certificates and requests that are already running, such as `watch` or `exec`. The check is not configured when the [`apiserver.authz.webhookURL`](/modules/control-plane-manager/configuration.html#parameters-apiserver-authz-webhookurl) parameter of the `control-plane-manager` module is set.

### Default access list for each role

Each next role inherits permissions from the previous roles. A role block shows only the permissions added by that role.

The list below includes:

- standard permissions from the basic role-based model (k8s permissions);
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
    - deckhouse.io/nodeextensionrequests
    - deckhouse.io/nodegroups
    - deckhouse.io/nodeoperations
    - deckhouse.io/openstackinstanceclasses
    - deckhouse.io/operationpolicies
    - deckhouse.io/projectnamespaces
    - deckhouse.io/projectrolebindings
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
    - network.deckhouse.io/waypointinstances
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
get,patch:
    - pods/resize
write:
    - apps/deployments
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - cert-manager.io/certificates
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
    - network.deckhouse.io/waypointinstances
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
create:
    - serviceaccounts/token
create,patch,update:
    - pods
delete,deletecollection:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - apps/replicasets
    - cert-manager.io/certificaterequests
    - extensions/replicasets
read-write:
    - deckhouse.io/authorizationrules
write:
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - cert-manager.io/issuers
    - deckhouse.io/applications
    - extensions.istio.io/wasmplugins
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - telemetry.istio.io/telemetries
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterEditor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create,delete,patch:
    - deckhouse.io/nodeoperations
delete,deletecollection:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - cert-manager.io/certificaterequests
get,list:
    - templates.internal.deckhouse.io/nodeconfigtemplates
patch,update:
    - nodes
read:
    - deckhouse.io/containerdintegritypolicies
    - deckhouse.io/ingressistiocontrollers
    - deckhouse.io/istiofederations
    - deckhouse.io/istiomulticlusters
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
    - cert-manager.io/issuers
    - deckhouse.io/applications
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/nodeextensionrequests
    - deckhouse.io/nodegroups
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
create:
    - authorization.deckhouse.io/bulksubjectaccessreviews
    - authorization.deckhouse.io/roleaccessreports
    - authorization.deckhouse.io/subjectaccessreports
    - deckhouse.io/dexauthenticators/allow-access-to-kubernetes
    - deckhouse.io/dexclients/allow-access-to-kubernetes
get,list,patch,update,watch:
    - control-plane.deckhouse.io/controlplanenodes
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
proxy:
    - nodes
read:
    - control-plane.deckhouse.io/controlplaneoperations
    - deckhouse.io/modulesettingsdefinitions
    - deckhouse.io/nodestaticpodrequests
    - nfd.k8s-sigs.io/nodefeaturegroups
    - nfd.k8s-sigs.io/nodefeaturerules
    - nfd.k8s-sigs.io/nodefeatures
read-write:
    - deckhouse.io/clusterauthorizationrules
    - deckhouse.io/deckhousereleases
    - deckhouse.io/dexproviderchecks
    - deckhouse.io/dexproviders
    - deckhouse.io/groups
    - deckhouse.io/moduleconfigs
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/nodeusers
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/sshcredentials
    - deckhouse.io/useraccounts
    - deckhouse.io/useroperations
    - deckhouse.io/users
    - nodes/configz
    - nodes/healthz
    - nodes/log
    - nodes/metrics
    - nodes/pods
    - nodes/proxy
    - nodes/stats
update:
    - namespaces/finalize
write:
    - cilium.io/ciliumclusterwidenetworkpolicies
    - cilium.io/ciliumnetworkpolicies
    - config.gatekeeper.sh/configs
    - connection.gatekeeper.sh/connections
    - constraints.gatekeeper.sh/*
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/awsinstanceclasses
    - deckhouse.io/azureinstanceclasses
    - deckhouse.io/clusterprojectrolebindings
    - deckhouse.io/containerdintegritypolicies
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
    - deckhouse.io/securitypolicyexceptions
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

You can get additional list of access rules for module role from cluster ([rules added by the administrator](usage.html#customizing-rights-of-high-level-roles) and non-default rules of other Deckhouse modules):

```bash
D8_ROLE_NAME=Editor
d8 k get clusterrole -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```
