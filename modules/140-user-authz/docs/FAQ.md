---
title: "The user-authz module: FAQ"
---

## How do I create a user?

[Creating a user](usage.html#creating-a-user).

<div style="height: 0;" id="how-do-i-limit-user-rights-to-specific-namespaces-obsolete-role-based-model"></div>

## How do I limit user rights to specific namespaces?

To limit a user's rights to specific namespaces in the primary role-based model, use `RoleBinding` with the [namespace role](./#namespace-roles) that has the appropriate level of access. [Example...](usage.html#example-of-assigning-administrative-rights-to-a-user-within-a-namespace).

In the legacy role-based model, use the `namespaceSelector` or `limitNamespaces` (deprecated) parameters in the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule) CR.

## What if there are two ClusterAuthorizationRules matching to a single user?

In the example, the user `jane.doe@example.com` is in the `administrators` group. There are two cluster authorization rules:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: jane
spec:
  subjects:
    - kind: User
      name: jane.doe@example.com
  accessLevel: User
  namespaceSelector:
    labelSelector:
      matchLabels:
        env: review
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: Group
    name: administrators
  accessLevel: ClusterAdmin
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: env
        operator: In
        values:
        - prod
        - stage
```

1. `jane.doe@example.com` has the right to get and list any objects in the namespaces labeled `env=review`
2. `Administrators` can get, edit, list, and delete objects on the cluster level and in the namespaces labeled `env=prod` and `env=stage`.

Because `Jane Doe` matches two rules, some calculations will be made:

* `Jane Doe` will have the most powerful accessLevel across all matching rules — `ClusterAdmin`.
* The `namespaceSelector` options will be combined, so that Jane will have access to all the namespaces labeled with `env` label of the following values: `review`, `stage`, or `prod`.

{% alert level="warning" %}
If there is a rule without the `namespaceSelector` option and `limitNamespaces` deprecated option, it means that all namespaces are allowed excluding system namespaces, which will affect the resulting limit namespaces calculation.
{% endalert %}

## Can the legacy and the primary role-based models be used at the same time?

Yes. Both models ultimately boil down to the standard Kubernetes RBAC mechanism, and RBAC is a permissive model: permissions from all sources are **summed up**. If an action is allowed by at least one source — a `ClusterAuthorizationRule`, an `AuthorizationRule`, a `RoleBinding` to an primary-model role, or a `ProjectRoleBinding` — it will be allowed. Nothing needs to be "switched over": you can keep the existing `ClusterAuthorizationRule` objects and gradually add primary-model role bindings.

The only exception is the multitenancy mode ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)). If a user has a `ClusterAuthorizationRule` with a namespace restriction (`limitNamespaces` or `namespaceSelector`), that restriction acts as a **hard boundary**: requests to namespaces outside the list are denied even if the user has a `RoleBinding` there. See [the module description](./#rolebinding-car) for details. If a user needs combined access, use an `AuthorizationRule` instead of a `ClusterAuthorizationRule`, or do not set a namespace restriction in the CAR.

## How do I get an equivalent of the ClusterAdmin and SuperAdmin roles in the primary model?

There is no single-object counterpart of the legacy model's `ClusterAdmin` and `SuperAdmin` roles in the primary model — it deliberately separates platform administration (system roles) from application access (namespace and project roles). The equivalent is assembled from **two bindings**: a `ClusterRoleBinding` to a system role and a [ClusterProjectRoleBinding](../multitenancy-manager/cr.html#clusterprojectrolebinding) to a project role (the latter applies in all projects, including those created later).

Approximate level mapping:

| Legacy model role | Primary model equivalent |
|--------------------|-------------------------------|
| `User` | `d8:namespace:viewer` (via `RoleBinding` or `ProjectRoleBinding`) |
| `PrivilegedUser` | `d8:namespace:user` |
| `Editor` | `d8:namespace:manager` |
| `Admin` | `d8:namespace:admin` |
| `ClusterEditor` | `d8:system:manager` (roughly; the scope is the platform and system namespaces) |
| `ClusterAdmin` | `d8:system:manager` + a `ClusterProjectRoleBinding` to `d8:project:admin` |
| `SuperAdmin` | `d8:system:superadmin` + a `ClusterProjectRoleBinding` to `d8:project:superadmin` |

An example for `ClusterAdmin` (the `k8s-admins` group):

```yaml
# Platform: DKP module configuration, cluster-wide resources, system namespaces.
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

Specifics:

- With [automatic project creation](../multitenancy-manager/configuration.html#parameters-allownamespaceswithoutprojects) enabled, every user namespace is a project, so the "system role + `ClusterProjectRoleBinding`" pair covers both the platform and all user namespaces. Only the `default` namespace is not covered — it is neither a project nor a system namespace.
- You cannot create a custom "all permissions" role (`apiGroups: ["*"], resources: ["*"], verbs: ["*"]`): such a role would also grant project management permissions and is rejected by the [built-in protections](./#built-in-protections-of-the-role-model). If you need truly unrestricted access to the whole API (outside the platform role model), use a `ClusterRoleBinding` to the built-in Kubernetes `cluster-admin` role — only someone who already has such permissions can assign it.

## How do I grant a user access to the resources of one module only?

A typical request: a user in a namespace should only work with the resources of one module (for example, only with virtual machines) without seeing the other resources (`Pod`, `Deployment`, etc.).

Every DKP module ships separate capabilities for its resources, so such access is granted without writing RBAC rules. Assemble a [custom role](#creating-a-custom-namespace-or-project-role) that aggregates only the capabilities of the desired module (a selector by the `module` label):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:virtualization-only
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"   # Allows using the role in a RoleBinding inside projects.
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: capability
        rbac.deckhouse.io/scope: namespace
        module: virtualization
rules: []
```

Grant the role via a `RoleBinding` in the desired namespace or via a [ProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) across the whole project. The user will get access only to the module's resources — the standard Kubernetes resources will not be visible to them.

Outside of projects the same can be done even simpler — by binding the module capability directly, without creating a role:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: virtualization-view
  namespace: my-namespace
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:namespace-capability:virtualization:view
  apiGroup: rbac.authorization.k8s.io
```

Note: inside **project** namespaces a plain `RoleBinding` may only reference roles [available to the project](../multitenancy-manager/usage.html#which-roles-are-available-in-a-rolebinding-inside-a-project) — capabilities are not among them by default, so for projects use the custom-role variant (the `rbac.deckhouse.io/delegatable: "true"` label in the example above is exactly what makes it available) or a `ProjectRoleBinding`.

## How do I extend a role or create a new one?

[The primary role model](./#primary-role-based-model) is based on the aggregation principle; it compiles smaller roles into larger ones,
thus providing easy ways to enhance the model with custom roles.

### Creating a new role subsystem

Suppose that the current subsystems do not fit the role distribution in the company. You need to create a new [subsystem](./#subsystems-of-the-role-based-model)
that includes roles from the `deckhouse` subsystem, the `kubernetes` subsystem and the user-authn module.

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

- The hook will use this namespace role when creating `RoleBinding` in the module namespaces:

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

- This one aggregates all the system-scope capabilities defined for the user-authn module:

  ```yaml
   rbac.deckhouse.io/scope: system
   module: user-authn
  ```

This way, your role will combine permissions of the `deckhouse` subsystem, `kubernetes` subsystem, and the user-authn module.

Notes:

* Custom roles and capabilities must be named with the `d8:custom:` prefix (the rest of the `d8:` prefix space is reserved for Deckhouse built-in objects). The name must agree with the declared scope: a subsystem role is `d8:custom:<subsystem>:<name>` (the segment is the subsystem itself, as in the example above), a namespace or project role is `d8:custom:namespace:<name>` or `d8:custom:project:<name>`, and a capability is `d8:custom:<scope>-capability:<name>`. A name that disagrees with the `rbac.deckhouse.io/scope` label is rejected.
* Namespace roles (`RoleBinding` with `d8:namespace:<level>`) will be created in the namespaces of the aggregated subsystems' modules, the level is specified by the `rbac.deckhouse.io/use-role` label.

### Extending the custom role

Suppose a new cluster CRD object, MySuperResource, has been created in the cluster (a manage role example), and you need to extend the custom role from the example above to include the permissions to interact with this resource.

First, you have to add a new selector to the role:

```yaml
rbac.deckhouse.io/aggregate-to-mycustom-as: manager
```

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

 Next, you need to create a new capability and define permissions for the new resource, e. g., the read-only permission:

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

The capability will update the subsystem role to include its rights, so that the role bearer will be able to view the new object.

Notes:

* Custom capabilities must be named with the `d8:custom:` prefix; the rest of the name is not restricted, but we recommend following the same pattern for the sake of readability.

### Extending the existing subsystem roles

To extend an existing role, follow the procedure outlined in the section above. Be sure to change the labels and the role name!

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

### Extending subsystem roles and adding a new namespace

If you need to create a new namespace (to create a namespace role binding in it by the hook), you only need to add one label:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

This label instructs the hook to create a `RoleBinding` with the namespace role in this namespace:

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

The hook monitors `ClusterRoleBinding`, and when creating a bindings, it loops through all the system and subsystem roles to find all the aggregated capabilities by checking the aggregation rule. It then fetches the namespace from the `rbac.deckhouse.io/namespace` label and creates a `RoleBinding` with the namespace role in that namespace.

The hook only looks at objects that declare `rbac.deckhouse.io/scope: system` or `subsystem`. A capability without that label still contributes its rules to the role through aggregation, but its `rbac.deckhouse.io/namespace` label is never read, and no `RoleBinding` appears in the namespace.

### Extending the existing namespace roles

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

### Creating a custom namespace or project role

Sometimes the built-in level ladder does not fit: for example, you need a "developer" role — full view of the namespace plus reading logs, but without the right to change quotas or RBAC. Such a role is assembled from ready-made capabilities, without writing RBAC rules by hand.

The rules for custom roles:

- the name must start with `d8:custom:` (for example, `d8:custom:namespace:developer`);
- the role must carry the `rbac.deckhouse.io/kind: custom-role` label;
- the role **cannot contain its own rules** (`rules`) — it may only aggregate capabilities via `aggregationRule`. Permissions are described in separate capabilities, so the contents of the role stay transparent;
- a single role cannot aggregate capabilities of the user-facing scopes (`namespace`, `project`) together with the administrative ones (`system`, subsystems) — such a role is rejected.

An example: a role that includes everything `d8:namespace:viewer` can do, plus one specific capability (connecting to pods), selected precisely by its unique `rbac.deckhouse.io/capability` label:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:developer
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
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

To list all available capabilities and their unique names:

```shell
d8 k get clusterroles -l rbac.deckhouse.io/kind=capability \
  -o custom-columns='NAME:.metadata.name,CAPABILITY:.metadata.labels.rbac\.deckhouse\.io/capability'
```

The resulting role is assigned exactly like a built-in one: via a `RoleBinding` in a namespace or via a [ProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) across a whole project (for project roles, use `rbac.deckhouse.io/scope: project` and aggregate `aggregate-to-project-as`). It cannot be assigned via a `ClusterRoleBinding` — just like the built-in roles of these scopes.

> You can also assemble such a role without YAML — with the access grant wizard in the Deckhouse Console web interface: it shows the available capabilities, builds a role out of them, and immediately creates the required binding.

## How do I migrate custom roles to the new scheme in DKP 1.78?

{% alert level="warning" %}
Custom roles and capabilities get no compatibility aliases, unlike the [built-in roles](./#deprecated-role-names). Until every one of them is migrated, the upgrade is held back by the `legacyRBACv2CustomRolesCount` release requirement, and the `D8UserAuthzLegacyRBACv2CustomRoleFound` alert fires.
{% endalert %}

Along with the role renaming ([name mapping](./#deprecated-role-names)), the label scheme that drives aggregation has changed.

Custom roles created with the old scheme **stop aggregating permissions** after the upgrade: the built-in capabilities are relabeled, and the old aggregation selectors (for example, `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<subsystem>-as`) no longer match them. No compatibility aliases are created for custom roles — they must be updated manually.

Mapping between the old and the new scheme:

| Before (old scheme) | After (new scheme) |
|---------------------|--------------------|
| Arbitrary role name (for example, `custom:manage:mycustom:manager`) | Mandatory `d8:custom:` prefix (for example, `d8:custom:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` or `use` on your role | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` or `use` on your capability | `rbac.deckhouse.io/kind: custom-capability`, name prefixed with `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/aggregate-to-all-as: <level>` | `rbac.deckhouse.io/aggregate-to-system-as: <level>` |
| Aggregation selector: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` | Only `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` |
| Selector for use permissions: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <level>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <level>` |
| Per-module selector: `rbac.deckhouse.io/kind: manage` + `module: <module>` | `rbac.deckhouse.io/scope: system` + `module: <module>` |

The names of the built-in capabilities changed as well (no aliases):

* `d8:manage:permission:module:<module>:view|edit` → `d8:system-capability:<module>:view|edit`
* `d8:use:capability:module:<module>:view|edit` → `d8:namespace-capability:<module>:view|edit`

Aggregation selectors match labels, not names, so updating the selectors is enough. Do not bind capabilities directly.

Find everything that still has to be migrated:

```shell
d8 k get clusterroles -o json | jq -r '.items[] | select((.metadata.name | startswith("custom:")) and ((.metadata.labels["rbac.deckhouse.io/kind"] // "" | IN("manage", "use")) or ([.aggregationRule.clusterRoleSelectors[]?.matchLabels["rbac.deckhouse.io/kind"] // ""] | any(IN("manage", "use"))))) | .metadata.name'
```

### Migration steps

Do the following:

1. Create a new version of a custom role — with the `d8:custom:` prefix, the `rbac.deckhouse.io/kind: custom-role` label, and the new aggregation selectors. See the before and after examples below.
1. Recreate your capabilities with the `rbac.deckhouse.io/kind: custom-capability` label and the `d8:custom:` name prefix.
1. Recreate the RoleBinding and ClusterRoleBinding objects pointing at the old role with the new name in the `roleRef` field. This field is immutable, so a binding has to be deleted and created anew.
1. After you ensure the new bindings are correct, delete the old roles and capabilities.

### Examples

#### Custom role before and after

The following is a configuration example of a role combining the permissions of the `deckhouse` and `kubernetes` subsystems and the `user-authn` module.

* Before (old scheme):

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

* After (new scheme):

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

What changed:

- The name got the mandatory `d8:custom:` prefix
- `rbac.deckhouse.io/kind: manage` → `rbac.deckhouse.io/kind: custom-role`
- `rbac.deckhouse.io/level: subsystem` → `rbac.deckhouse.io/scope: subsystem`
- `rbac.deckhouse.io/aggregate-to-all-as` → `rbac.deckhouse.io/aggregate-to-system-as`
- The `rbac.deckhouse.io/kind: manage` label is removed from the aggregation selectors
- All system permissions of a module are now selected with `rbac.deckhouse.io/scope: system` + `module: <module>`

#### Custom capability before and after

The following is an example configuration of a capability, which grants read access to the MySuperResource resource and is aggregated into the role from the example above (its `aggregationRule` field must contain the `rbac.deckhouse.io/aggregate-to-mycustom-as: manager` selector).

* Before (old scheme):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: custom:manage:permission:mycustom:superresource:view
    labels:
      rbac.deckhouse.io/kind: manage
      rbac.deckhouse.io/aggregate-to-custom-as: manager
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

* After (new scheme):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: d8:custom:capability:mycustom:superresource:view
    labels:
      rbac.deckhouse.io/kind: custom-capability
      rbac.deckhouse.io/aggregate-to-mycustom-as: manager
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

### Labels and annotations: before and after

Labels on ClusterRole objects:

| Label | Before | After | Purpose |
|-------|--------|-------|---------|
| `rbac.deckhouse.io/kind` | `manage` or `use` | `custom-role` / `custom-capability` for your own objects; `role` / `capability` on built-in ones (reserved) | The object type in the role model. Mandatory: objects without it are not processed |
| `rbac.deckhouse.io/level` | `all` \| `subsystem` \| `module` | Removed | The old role level; replaced by the `scope` label |
| `rbac.deckhouse.io/scope` | — | `system` \| `subsystem` \| `namespace` | The scope of a role or capability |
| `rbac.deckhouse.io/subsystem` | Subsystem name | Unchanged | The role's subsystem; used with `scope: subsystem` |
| `rbac.deckhouse.io/use-role` | A use-role level | A namespace-role level | Defines which namespace role is automatically granted to the holder of a system/subsystem role in the system namespaces of its modules (via automatically created RoleBinding objects) |
| `rbac.deckhouse.io/aggregate-to-all-as` | `<level>` | Renamed to `rbac.deckhouse.io/aggregate-to-system-as` | Aggregates the object into the system-wide role (`d8:system:<level>`) |
| `rbac.deckhouse.io/aggregate-to-<subsystem>-as` | Used in selectors together with `rbac.deckhouse.io/kind: manage` | Used in selectors on its own | Aggregates the object into the subsystem role of the given level |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as` | `<level>` (for use permissions) | Renamed to `rbac.deckhouse.io/aggregate-to-namespace-as` | Aggregates the object into the namespace role (`d8:namespace:<level>`) |
| `rbac.deckhouse.io/namespace` | Namespace | Unchanged | An additional namespace where a RoleBinding is automatically created for the role holders |
| `rbac.deckhouse.io/capability` | — | A unique capability name (for example, `system-capability.deckhouse.view`) | A machine-readable identifier of a built-in capability |
| `rbac.deckhouse.io/deprecated` | — | `"true"` on alias roles | The role is deprecated and will be removed; migrate the bindings to the new role |
| `module` | Module name | Unchanged | Marks a built-in object as belonging to a DKP module; handy in aggregation selectors together with `scope` |
| `heritage: deckhouse` | Platform object marker | Unchanged | Must not be set on custom objects |

Annotations on ClusterRole objects (the old scheme did not use annotations):

| Annotation | Purpose |
|------------|---------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | The displayed name and description of a role/capability in Russian (the platform sets them on built-in objects; you can set your own on custom ones) |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | Same in English |
| `rbac.deckhouse.io/deprecated-replaced-by` | Introduced in DKP 1.78 together with the new scheme. The aggregation rules of the previous roles will change so that these roles keep granting the same permissions as their new counterparts — existing bindings will not break. However, the previous roles are kept for one release only: during that time, the bindings must be migrated to the new roles. The annotation is set on every previous role and contains the name of the new role that is equivalent to it in terms of permissions — that is the role to migrate to |

### Adding a custom capability (in the new scheme)

A capability is a regular ClusterRole object with rules that is automatically included into the chosen role via an aggregation label. In the new scheme, a custom capability is created as follows:

1. Decide which role you want to extend: a namespace role, a subsystem role, the system role, or your own custom role.
1. Create a ClusterRole with the `d8:custom:` name prefix (for readability — `d8:custom:capability:<name>:<resource>:<action>`), the `rbac.deckhouse.io/kind: custom-capability` label, and the aggregation label of the target role:
   - `rbac.deckhouse.io/aggregate-to-namespace-as: <viewer|user|manager|admin|superadmin>`: Into the `d8:namespace:<level>` namespace role.
   - `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <viewer|manager|superadmin>`: Into the `d8:subsystem:<subsystem>:<level>` subsystem role.
   - `rbac.deckhouse.io/aggregate-to-system-as: <viewer|manager|superadmin>`: Into the `d8:system:<level>` system role.
   - `rbac.deckhouse.io/aggregate-to-<your subsystem name>-as: <level>`: Into your own custom role (its `aggregationRule` field must contain such a selector).
1. Define the permissions in `rules`.

Kubernetes aggregates the rules automatically: right after the capability is created, its permissions appear for all holders of the target role. You can verify the result with `d8 k auth can-i --as <user>` or by inspecting the resulting role rules: `d8 k get clusterrole <role> -o yaml`.

For configuration examples, refer to the ["Custom role before and after"](#custom-role-before-and-after) and ["Custom capability before and after"](#custom-capability-before-and-after) subsections.

## How do I rename a built-in role?

The permissions of built-in roles cannot be changed, but their display title and description can — for example, so that the interface shows them in your company's terms. Add the `custom.meta.deckhouse.io/title` and `custom.meta.deckhouse.io/description` annotations to the role:

```shell
d8 k annotate clusterrole d8:namespace:admin \
  custom.meta.deckhouse.io/title='Team administrator' \
  custom.meta.deckhouse.io/description='Manages resources and access in the team namespace'
```

This is the only modification allowed for objects with the `d8:` prefix (except `d8:custom:*`): an attempt to change the rules, aggregation, or labels of a built-in role is rejected.

## How do I find out who has access to a resource?

With the multitenancy mode enabled ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)), a reverse authorization query is available — the `WhoCan` resource. It answers the question "who can perform action X on resource Y?" and returns the list of users, groups, and ServiceAccounts:

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: WhoCan
metadata:
  name: who-can-create-networkpolicies
spec:
  resourceAttributes:
    namespace: my-namespace
    verb: create
    group: networking.k8s.io
    resource: networkpolicies
EOF
```

The answer is returned in the `status` field (`users`, `groups`, `serviceAccounts`) directly in the command output; the object is not stored anywhere.

The right to create `WhoCan` queries is granted by the `d8:user-authz:who-can-checker` cluster role. It is intentionally not bound to anyone by default: the query result discloses access subjects across all namespaces, so grant it only to trusted administrators via a `ClusterRoleBinding`.

## How do I find out what a specific user, group, or ServiceAccount is allowed to do?

With the multitenancy mode enabled ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)), the `SubjectAccessReport` resource is available — the counterpart of `WhoCan`. It answers the question "what is this subject allowed to do" and returns a ready-made report: which roles are granted through which bindings, which actions on which resources are allowed cluster-wide and in every namespace, and where each permission comes from.

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: SubjectAccessReport
metadata:
  name: what-can-jane-do
spec:
  subject:
    kind: User
    name: jane@example.com
EOF
```

The `spec.subject.kind` field accepts `User`, `Group`, and `ServiceAccount` (for the latter, `spec.subject.namespace` is required). If `spec.subject` is omitted, the report is built for the caller.

Report specifics:

- The user's groups are resolved automatically from the [Group](../user-authn/cr.html#group) catalog, including nested ones: if the user belongs to group `B` and `B` belongs to `A`, the permissions of `A` are included as well. The groups taken into account are returned in `status.subject.groups`. Groups that are not in the catalog (for example, the ones coming from an external identity provider) can be passed in `spec.groups`.
- Every source of a permission (`status.scopes[].resources[].sources[]`) carries a `matchedBy` field showing whether the permission was granted to the subject personally or through a group, which makes it possible to view the access with group-derived grants excluded.
- Namespaces with identical access are merged into a single section (`status.scopes[].namespaces`), so a project with a dozen namespaces does not turn into a dozen identical tables.
- Permissions granted by a `ClusterRoleBinding` are reported once, in the cluster-wide scope (`status.scopes[].cluster: true`), since they apply in every namespace.
- The report can be limited to specific namespaces with `spec.namespaces`.

The report is built from RBAC data and does not account for admission webhook restrictions: for example, editing and deleting system resources is denied to everyone below the `superadmin` level. Such cases are flagged in `status.scopes[].caveat`. If the subject's permissions are limited by a `ClusterAuthorizationRule`, this is reported in `status.notes`.

The right to build a report about **another** subject is granted by the `d8:user-authz:subject-access-checker` cluster role. Like `who-can-checker`, it is intentionally not bound to anyone by default: the report discloses the full permission map of the subject, including other namespaces. A report about oneself is available to every authenticated user and requires no extra permissions.

## How does a user see the list of namespaces available to them?

With the multitenancy mode enabled ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)), the namespace list is filtered automatically: the `d8 k get namespaces` command returns to a user only the namespaces they have access to — via any of the mechanisms (role bindings, `ProjectRoleBinding`/`ClusterProjectRoleBinding`, `ClusterAuthorizationRule`/`AuthorizationRule`). A user does not see foreign namespaces and cannot learn about their existence from the list.

The same list is served by the read-only `accessiblenamespaces` resource — any authenticated user can query it **for themselves**:

```shell
d8 k get accessiblenamespaces
```

This is convenient for scripts and interfaces: there is no need to iterate over namespaces checking access to each one.
