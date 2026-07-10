---
title: "The user-authz module: FAQ"
---

## How do I create a user?

[Creating a user](usage.html#creating-a-user).

<div style="height: 0;" id="how-do-i-limit-user-rights-to-specific-namespaces-obsolete-role-based-model"></div>

## How do I limit user rights to specific namespaces?

To limit a user's rights to specific namespaces in the experimental role-based model, use `RoleBinding` with the [use role](./#use-roles) that has the appropriate level of access. [Example...](usage.html#example-of-assigning-administrative-rights-to-a-user-within-a-namespace).

In the current role-based model, use the `namespaceSelector` or `limitNamespaces` (deprecated) parameters in the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule) CR.

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

## How do I extend a role or create a new one?

[The experimental role model](./#experimental-role-based-model) is based on the aggregation principle; it compiles smaller roles into larger ones,
thus providing easy ways to enhance the model with custom roles.

### Creating a new role subsystem

Suppose that the current subsystems do not fit the role distribution in the company. You need to create a new [subsystem](./#subsystems-of-the-role-based-model)
that includes roles from the `deckhouse` subsystem, the `kubernetes` subsystem and the user-authn module.

To meet this need, create the following role:

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

The labels for the new role listed at the top suggest that:

- The hook will use this use role:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- The role must be treated as a managed one:

  ```yaml
  rbac.deckhouse.io/kind: manage
  ```

  > Note that this label is mandatory.
  
- The role is a subsystem one, and it shall be handled accordingly:

  ```yaml
  rbac.deckhouse.io/level: subsystem
  ```

- There is a subsystem for which the role is responsible:

  ```yaml
  rbac.deckhouse.io/subsystem: custom
  ```

- The `manage:all` role can aggregate this role:

  ```yaml
  rbac.deckhouse.io/aggregate-to-all-as: manager
  ```

Then there are selectors that implement aggregation:

- This one aggregates the manager role from the `deckhouse` subsystem:

  ```yaml
  rbac.deckhouse.io/kind: manage
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- This one aggregates all the rules defined for the user-authn module:

  ```yaml
   rbac.deckhouse.io/kind: manage
   module: user-authn
  ```

This way, your role will combine permissions of the `deckhouse` subsystem, `kubernetes` subsystem, and the user-authn module.

Notes:

* There are no restrictions on role name, but we recommend following the same pattern for the sake of readability.
* Use-roles will be created in aggregate subsystems and the module namespace, the role type is specified by the label.

### Extending the custom role

Suppose a new cluster CRD object, MySuperResource, has been created in the cluster (a manage role example), and you need to extend the custom role from the example above to include the permissions to interact with this resource.

First, you have to add a new selector to the role:

```yaml
rbac.deckhouse.io/kind: manage
rbac.deckhouse.io/aggregate-to-custom-as: manager
```

This selector would enable roles to be aggregated to a new subsystem by specifying this label. After adding the new selector, the role will look as follows:

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

 Next, you need to create a new role and define permissions for the new resource, e. g., the read-only permission:

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

The role will update the subsystem role to include its rights, so that the role bearer will be able to view the new object.

Notes:

* There are no restrictions on capability names, but we recommend following the same pattern for the sake of readability.

### Extending the existing manage subsystem roles

To extend an existing role, follow the procedure outlined in the section above. Be sure to change the labels and the role name!

For example, here's how you can extend the manager role from the `deckhouse`(`d8:manage:deckhouse:manager`) subsystem:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
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

This way, the new role will extend the `d8:manage:deckhouse:manager` role.

### Extending manage subsystem roles and adding a new namespace

If you need to create a new namespace (to create a use role in it by the hook), you only need to add one label:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

This label instructs the hook to create a use role in this namespace:

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

The hook monitors `ClusterRoleBinding`, and when creating a bindings, it loops through all the manage roles to find all the aggregated roles by checking the aggregation rule. It then fetches the namespace from the `rbac.deckhouse.io/namespace` label and creates a use role in that namespace.

### Extending the existing use roles

If the resource belongs to a namespace, you need to extend the use role instead of the manage role. The only difference is the labels and the name:

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

This role will be added to the `d8:use:role:user:kubernetes` role.

## How do I migrate my custom roles to the new scheme?

{% alert level="warning" %}
This section describes [the role model renaming](./#migration-to-the-new-role-names) that will take effect in one of the upcoming DKP releases. In the current release, custom roles keep working under the old scheme described above.
{% endalert %}

Along with the role renaming ([name mapping](./#migration-to-the-new-role-names)), the label scheme that drives aggregation will change. Custom roles created with the old scheme **will stop aggregating permissions** after upgrading to the release with the renaming: the built-in capabilities will be relabeled, and the old aggregation selectors (e.g., `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<subsystem>-as`) will no longer match them. No compatibility aliases are created for custom roles — they must be updated manually.

Mapping between the old and the new scheme:

| Before (old scheme) | After (new scheme) |
|---------------------|--------------------|
| Role name — arbitrary (e.g., `custom:manage:mycustom:manager`) | Mandatory `d8:custom:` prefix (e.g., `d8:custom:subsystem:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` or `use` on your role | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` or `use` on your capability | `rbac.deckhouse.io/kind: custom-capability`, name prefixed with `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/aggregate-to-all-as: <level>` | `rbac.deckhouse.io/aggregate-to-system-as: <level>` |
| Aggregation selector: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` | Only `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` |
| Selector for use permissions: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <level>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <level>` |
| Per-module selector: `rbac.deckhouse.io/kind: manage` + `module: <module>` | `rbac.deckhouse.io/scope: system` + `module: <module>` |

The names of the built-in capabilities will change as well (no aliases): `d8:manage:permission:module:<module>:view|edit` → `d8:system-capability:<module>:view|edit`, `d8:use:capability:module:<module>:view|edit` → `d8:namespace-capability:<module>:view|edit`. Aggregation selectors match labels, not names, so updating the selectors is enough; do not bind capabilities directly.

### Example: a custom role before and after

A role combining the permissions of the `deckhouse` and `kubernetes` subsystems and the `user-authn` module.

Before (old scheme):

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

After (new scheme):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:subsystem:mycustom:manager
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

- the name got the mandatory `d8:custom:` prefix;
- `rbac.deckhouse.io/kind: manage` → `rbac.deckhouse.io/kind: custom-role`;
- `rbac.deckhouse.io/level: subsystem` → `rbac.deckhouse.io/scope: subsystem`;
- `rbac.deckhouse.io/aggregate-to-all-as` → `rbac.deckhouse.io/aggregate-to-system-as`;
- the `rbac.deckhouse.io/kind: manage` label is removed from the aggregation selectors;
- all system permissions of a module are now selected with `rbac.deckhouse.io/scope: system` + `module: <module>`.

### Example: a custom capability before and after

The capability grants read access to the `MySuperResource` resource and is aggregated into the role from the example above (its `aggregationRule` must contain the `rbac.deckhouse.io/aggregate-to-mycustom-as: manager` selector).

Before (old scheme):

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

After (new scheme):

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

Labels on `ClusterRole` objects:

| Label | Before | After | Purpose |
|-------|--------|-------|---------|
| `rbac.deckhouse.io/kind` | `manage` or `use` | `custom-role` / `custom-capability` — for your own objects; `role` / `capability` — on built-in ones (reserved) | The object type in the role model. Mandatory: objects without it are not processed |
| `rbac.deckhouse.io/level` | `all` \| `subsystem` \| `module` | Removed | The old role level; replaced by the `scope` label |
| `rbac.deckhouse.io/scope` | — | `system` \| `subsystem` \| `namespace` | The scope of a role or capability |
| `rbac.deckhouse.io/subsystem` | Subsystem name | Unchanged | The role's subsystem; used with `scope: subsystem` |
| `rbac.deckhouse.io/use-role` | A use-role level | A namespace-role level | Which namespace role is automatically granted to the holder of a system/subsystem role in the system namespaces of its modules (via automatically created `RoleBinding` objects) |
| `rbac.deckhouse.io/aggregate-to-all-as` | `<level>` | Renamed to `rbac.deckhouse.io/aggregate-to-system-as` | Aggregates the object into the system-wide role (`d8:system:<level>`) |
| `rbac.deckhouse.io/aggregate-to-<subsystem>-as` | Used in selectors together with `rbac.deckhouse.io/kind: manage` | Used in selectors on its own | Aggregates the object into the subsystem role of the given level |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as` | `<level>` (for use permissions) | Renamed to `rbac.deckhouse.io/aggregate-to-namespace-as` | Aggregates the object into the namespace role (`d8:namespace:<level>`) |
| `rbac.deckhouse.io/namespace` | Namespace | Unchanged | An additional namespace where a `RoleBinding` is automatically created for the role holders |
| `rbac.deckhouse.io/capability` | — | A unique capability name (e.g., `system-capability.deckhouse.view`) | A machine-readable identifier of a built-in capability |
| `rbac.deckhouse.io/deprecated` | — | `"true"` on alias roles | The role is deprecated and will be removed; migrate the bindings to the new role |
| `module` | Module name | Unchanged | Marks a built-in object as belonging to a DKP module; handy in aggregation selectors together with `scope` |
| `heritage: deckhouse` | Platform object marker | Unchanged | Must not be set on your own objects |

Annotations on `ClusterRole` objects (the old scheme did not use annotations):

| Annotation | Purpose |
|------------|---------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | The displayed name and description of a role/capability in Russian (the platform sets them on built-in objects; you can set your own on custom ones) |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | Same in English |
| `rbac.deckhouse.io/deprecated-replaced-by` | On deprecated alias roles: the name of the new role the bindings should be migrated to |

### How do I add my own custom capability (in the new scheme)?

A capability is a regular `ClusterRole` with rules that is automatically included into the chosen role via an aggregation label. In the new scheme, a custom capability is created as follows:

1. Decide which role you want to extend: a namespace role, a subsystem role, the system role, or your own custom role.
1. Create a `ClusterRole` with the `d8:custom:` name prefix (for readability — `d8:custom:capability:<name>:<resource>:<action>`), the `rbac.deckhouse.io/kind: custom-capability` label, and the aggregation label of the target role:
   - `rbac.deckhouse.io/aggregate-to-namespace-as: <viewer|user|manager|admin|superadmin>` — into the `d8:namespace:<level>` namespace role;
   - `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <viewer|manager|superadmin>` — into the `d8:subsystem:<subsystem>:<level>` subsystem role;
   - `rbac.deckhouse.io/aggregate-to-system-as: <viewer|manager|superadmin>` — into the `d8:system:<level>` system role;
   - `rbac.deckhouse.io/aggregate-to-<your subsystem name>-as: <level>` — into your own custom role (its `aggregationRule` must contain such a selector).
1. Define the permissions in `rules`.

Kubernetes aggregates the rules automatically: right after the capability is created, its permissions appear for all holders of the target role. You can verify the result with `d8 k auth can-i --as <user>` or by inspecting the resulting role rules: `d8 k get clusterrole <role> -o yaml`.

Ready-made examples in the new scheme are provided above, in the "[Example: a custom role before and after](#example-a-custom-role-before-and-after)" and "[Example: a custom capability before and after](#example-a-custom-capability-before-and-after)" subsections.

### Migration steps

After upgrading to the release with the renaming:

1. Create a new version of your role — with the `d8:custom:` prefix, the `rbac.deckhouse.io/kind: custom-role` label, and the new aggregation selectors (see the before/after example above).
1. Recreate your capabilities with the `rbac.deckhouse.io/kind: custom-capability` label and the `d8:custom:` name prefix.
1. Recreate the `RoleBinding`/`ClusterRoleBinding` objects pointing at the old role with the new name in `roleRef` (the `roleRef` field is immutable, so a binding has to be deleted and created anew).
1. Delete the old role and the old custom capabilities.
