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

<div style="height: 0;" id="how-do-i-migrate-custom-roles-to-the-new-scheme-in-dp-178"></div>
<div style="height: 0;" id="how-do-i-migrate-custom-roles-to-the-new-scheme-in-dkp-178"></div>

## How do I migrate custom roles to the new scheme?

Along with the [role renaming](./#migration-to-the-new-role-names-in-dp-178), the label scheme that drives aggregation changes.

{% alert level="warning" %}
Freeze the custom roles of the old scheme while the built-in capabilities still carry the old labels. A role that has already lost its permissions cannot be frozen with them.
{% endalert %}

In the new scheme, the built-in capabilities do not carry the `rbac.deckhouse.io/kind: manage` and `rbac.deckhouse.io/kind: use` labels.
A custom role whose aggregation selector requires one of these labels (for example, `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<subsystem>-as`) then matches none of them.
The Kubernetes aggregation controller leaves such a role without rules, and its subjects lose the permissions.
No compatibility aliases are created for custom roles.

The new role model also changes the list of subsystems.
Its roles collect only the `system`, `namespace` and `project` lineages and the lineages of the `iam`, `security`, `cluster`, `delivery`, `network`, `storage`, `observability` and `managed-services` subsystems, and the modules label their capabilities only with these.
A selector that requires the aggregation label of another lineage then matches none of the built-in capabilities either.
Such a label is, for example, `rbac.deckhouse.io/aggregate-to-all-as`, the label of the `deckhouse`, `infrastructure`, `kubernetes` or `networking` subsystem, or the label of the subsystem of a module that moves to one of the new subsystems, such as `rbac.deckhouse.io/aggregate-to-virtualization-as`.
A lineage that a custom capability carries is an exception, because the selector keeps getting the rules of that capability after the update. A custom capability here is a ClusterRole without the `heritage: deckhouse` label and without an `aggregationRule` field.
So is the lineage of the role's own subsystem, from its `rbac.deckhouse.io/subsystem` label, because the custom capabilities of that subsystem carry it.
Neither exception applies to the `all`, `deckhouse`, `infrastructure`, `kubernetes` and `networking` lineages, which do not count as custom ones.

The `D8UserAuthzLegacyRBACv2CustomRoleFound` alert lists such roles.
It fires for every ClusterRole without the `heritage: deckhouse` label whose `aggregationRule` field has a selector that requires `rbac.deckhouse.io/kind: manage` or `use`, or such an aggregation label, whatever the name of the role.
It also fires for a ClusterRole without the `heritage: deckhouse` label that has an `aggregationRule` field and carries the `rbac.deckhouse.io/kind: manage` and `rbac.deckhouse.io/use-role` labels, whatever its selectors.
For every ClusterRoleBinding to such a role, Deckhouse creates RoleBinding objects in the namespaces of the modules. In the new role model it creates them only for roles with the `rbac.deckhouse.io/scope: system` or `subsystem` label, so the update deletes the RoleBinding objects of such a role, see [Freezing a role](#freezing-a-role).
To list such roles, use the command:

```shell
d8 k get clusterroles -o json | jq -r '
  def lineages: [((.matchLabels // {}) | keys[]), (.matchExpressions[]? | select(.operator == "In" or .operator == "Exists") | .key)]
    | map(capture("^rbac\\.deckhouse\\.io/aggregate-to-(?<lineage>.+)-as$").lineage);
  ([.items[] | select(.metadata.labels.heritage != "deckhouse" and .aggregationRule == null) | (.metadata.labels // {}) | keys[]
        | capture("^rbac\\.deckhouse\\.io/aggregate-to-(?<lineage>.+)-as$").lineage]) as $carried
  | .items[]
  | select(.metadata.labels.heritage != "deckhouse" and .aggregationRule != null)
  | (.metadata.labels["rbac.deckhouse.io/subsystem"] // "") as $own
  | select(
      (
        .metadata.labels["rbac.deckhouse.io/kind"] == "manage"
        and ((.metadata.labels["rbac.deckhouse.io/use-role"] // "") != "")
      )
      or (
        [
          .aggregationRule.clusterRoleSelectors[]?
          | (.matchLabels["rbac.deckhouse.io/kind"] // empty),
            (.matchExpressions[]? | select(.key == "rbac.deckhouse.io/kind" and .operator == "In") | .values[])
        ]
        | any(IN("manage", "use"))
      )
      or (
        [
          .aggregationRule.clusterRoleSelectors[]? | lineages[]
          | select((IN("system", "namespace", "project", "iam", "security", "cluster", "delivery", "network", "storage", "observability", "managed-services") | not)
              and (IN("all", "deckhouse", "infrastructure", "kubernetes", "networking") or (. != $own and (IN($carried[]) | not))))
        ]
        | length > 0
      )
    )
  | .metadata.name'
```

Mapping between the old and the new scheme:

| Before (old scheme) | After (new scheme) |
|---------------------|--------------------|
| Arbitrary role name (for example, `custom:manage:mycustom:manager`) | Mandatory `d8:custom:` prefix, and the name of the subsystem for a subsystem role (for example, `d8:custom:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` or `use` on your role | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` or `use` on your capability | `rbac.deckhouse.io/kind: custom-capability`, name prefixed with `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/aggregate-to-all-as: <level>` on your capability | `rbac.deckhouse.io/aggregate-to-system-as: <level>` |
| `rbac.deckhouse.io/aggregate-to-all-as: <level>` on your role | No label. A custom role cannot be aggregated into `d8:system:<level>`, so bind it to the subjects of `d8:system:<level>` with a ClusterRoleBinding |
| Aggregation selector: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` | Only `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` |
| Labels of the `deckhouse`, `infrastructure` and `kubernetes` subsystems (`rbac.deckhouse.io/aggregate-to-deckhouse-as` and the others) | One label of the `cluster` subsystem, `rbac.deckhouse.io/aggregate-to-cluster-as`, or selectors by `rbac.deckhouse.io/capability` to keep the role as narrow as before, see [Selectors by a lineage the new role model does not collect](#selectors-by-a-lineage-the-new-role-model-does-not-collect) |
| Label of the `networking` subsystem, `rbac.deckhouse.io/aggregate-to-networking-as` | `rbac.deckhouse.io/aggregate-to-network-as` |
| The `user-authz`, `user-authn` and `multitenancy-manager` modules with the labels of the `security` and `deckhouse` subsystems | The label of the `iam` subsystem, `rbac.deckhouse.io/aggregate-to-iam-as` |
| Label of the subsystem of a module that moves to one of the new subsystems, such as `rbac.deckhouse.io/aggregate-to-virtualization-as` | Selectors by `rbac.deckhouse.io/capability`, see [Selectors by a lineage the new role model does not collect](#selectors-by-a-lineage-the-new-role-model-does-not-collect) |
| Selector for use permissions: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <level>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <level>` |
| Per-module selector: `rbac.deckhouse.io/kind: manage` + `module: <module>` | `rbac.deckhouse.io/scope: system` + `module: <module>` |

The names of the built-in capabilities change as well (no aliases):

* `d8:manage:permission:module:<module>:view|edit` → `d8:system-capability:<module>:view|edit`
* `d8:use:capability:module:<module>:view|edit` → `d8:namespace-capability:<module>:view|edit`
* `d8:manage:permission:subsystem:<subsystem>:manage_resources|view_resources` → `d8:subsystem-capability:<subsystem>:manage_resources|view_resources`, where the `kubernetes` subsystem becomes `cluster` and `networking` becomes `network`
* `d8:use:capability:kubernetes:<name>` → `d8:namespace-capability:kubernetes:<name>`

Aggregation selectors match labels, not names. Do not bind capabilities directly.

### Migration steps

The migration has two stages.
First, freeze the custom roles and relabel the custom capabilities while the built-in capabilities still carry the old labels. A frozen role keeps its own permissions under both schemes, and a relabeled capability stays in the roles it extends.
Then move to roles of the new scheme once the built-in capabilities carry the new labels. The explicit bindings that the freeze creates point at the `d8:use:role:<LEVEL>` roles, which the new scheme keeps only as deprecated aliases, and Deckhouse does not install an update that removes these aliases while such bindings exist.
Move them to `d8:namespace:<LEVEL>` after the update.

#### Freezing a role

Freeze every role that the `D8UserAuthzLegacyRBACv2CustomRoleFound` alert lists for its selectors.
Keep the order of the steps.
When a role with the `rbac.deckhouse.io/kind: manage` label loses its `aggregationRule` field, Deckhouse deletes the RoleBinding objects that it has created automatically for the subjects of the role. It also deletes those of the roles the role is aggregated into, in the namespaces that only this role leads to.

1. Bind `d8:use:role:<LEVEL>` explicitly in the namespaces of the automatic RoleBinding objects that the role leads to.

   For every ClusterRoleBinding to a role with the `rbac.deckhouse.io/kind: manage` and `rbac.deckhouse.io/use-role: <LEVEL>` labels, Deckhouse creates a RoleBinding to `d8:use:role:<LEVEL>` with the same subjects in each namespace named by the `rbac.deckhouse.io/namespace` label of the roles that the role aggregates.
   These RoleBinding objects have the `heritage: deckhouse` and `rbac.deckhouse.io/automated: "true"` labels and the `rbac.deckhouse.io/related-with: <CLUSTERROLEBINDING_NAME>` annotation.
   The following commands copy each of them for the ClusterRoleBinding objects to the `BOUND_ROLE` role. A copy has the same namespace, `roleRef`, `subjects` and `rbac.deckhouse.io/related-with` annotation, the `explicit:` name prefix, and none of these labels:

   ```shell
   BOUND_ROLE=<ROLE_NAME>
   d8 k get clusterrolebindings -o json > clusterrolebindings.json
   d8 k get rolebindings -A -l heritage=deckhouse,rbac.deckhouse.io/automated=true -o json > automated-rolebindings.json
   jq --arg role "$BOUND_ROLE" --slurpfile crbs clusterrolebindings.json '
     [$crbs[0].items[] | select(.roleRef.kind == "ClusterRole" and .roleRef.name == $role) | .metadata.name] as $names
     | {apiVersion: "v1", kind: "List", items: [.items[]
         | select(.metadata.annotations["rbac.deckhouse.io/related-with"] | IN($names[]))
         | {apiVersion, kind, metadata: {name: ("explicit:" + .metadata.name), namespace: .metadata.namespace,
             annotations: {"rbac.deckhouse.io/related-with": .metadata.annotations["rbac.deckhouse.io/related-with"]}},
            roleRef, subjects}]}' \
     automated-rolebindings.json > explicit-rolebindings.json
   d8 k create -f explicit-rolebindings.json
   ```

   Run the commands with `BOUND_ROLE` set to the name of the role if the role has the `rbac.deckhouse.io/use-role` label.
   Then run them again for every role with the `rbac.deckhouse.io/kind: manage` label that the role is aggregated into, whatever labels the role itself has. An example is `d8:manage:all:<LEVEL>`, which aggregates the roles with the `rbac.deckhouse.io/aggregate-to-all-as: <LEVEL>` label. The automatic RoleBinding objects of such a role also cover the namespaces that only this role leads to.

1. If the role is aggregated into another role, such as `d8:manage:all:<LEVEL>`, bind the role itself to the subjects of the ClusterRoleBinding objects to that other role.
   The built-in roles of the new scheme select other labels, so after the update those subjects keep the permissions of the frozen role only through such a binding:

   ```shell
   OUTER_ROLE=<OTHER_ROLE_NAME>
   jq --arg outer "$OUTER_ROLE" --arg role "<ROLE_NAME>" '
     {apiVersion: "v1", kind: "List", items: [.items[]
       | select(.roleRef.kind == "ClusterRole" and .roleRef.name == $outer)
       | {apiVersion, kind, metadata: {name: ("explicit:" + $role + ":" + .metadata.name)},
          roleRef: {apiGroup: "rbac.authorization.k8s.io", kind: "ClusterRole", name: $role}, subjects}]}' \
     clusterrolebindings.json > explicit-clusterrolebindings.json
   d8 k create -f explicit-clusterrolebindings.json
   ```

1. Remove the `aggregationRule` field from the role and keep its `rules` field:

   ```shell
   d8 k patch clusterrole <ROLE_NAME> --type=json -p '[{"op": "remove", "path": "/aggregationRule"}]'
   ```

   Kubernetes does not update the rules of a role without `aggregationRule`, so the role keeps the permissions it has at this moment, whatever labels the built-in capabilities carry.
   Deckhouse deletes the automatic RoleBinding objects of the role, and the explicit ones from the first step keep the access in those namespaces.
   The role disappears from the `D8UserAuthzLegacyRBACv2CustomRoleFound` alert.

Replace `<ROLE_NAME>` with the name of the role and `<OTHER_ROLE_NAME>` with the name of the role it is aggregated into.

If the role is applied from a Git repository (for example, by Argo CD or Flux), make the same change in its manifest.
Remove the `aggregationRule` field there and put the current rules of the role, from the output of `d8 k get clusterrole <ROLE_NAME> -o yaml`, into the `rules` field.
Otherwise, the next apply of the manifest restores `aggregationRule` and an empty `rules` field.
After the update, the `rbacv2-cluster-roles.deckhouse.io` webhook refuses every update of a ClusterRole that carries the aggregation label of a built-in lineage, such as `rbac.deckhouse.io/aggregate-to-all-as`, without the `rbac.deckhouse.io/kind: custom-capability` label, an apply of its manifest included.
A frozen role with such a label cannot be changed until it is replaced with a role of the new scheme, and the `D8UserAuthzForeignAggregationLabel` alert names it.

Deckhouse does not delete the explicit RoleBinding and ClusterRoleBinding objects.
When you revoke the access of a subject, delete them together with the ClusterRoleBinding they were copied for. For a RoleBinding, the `rbac.deckhouse.io/related-with` annotation names it, and the name of a ClusterRoleBinding ends with it.

The frozen role does not gain permissions for resources added later, such as the resources of a new module or of a new capability with matching labels.
Add such permissions to its `rules` field yourself, or replace the role with a role of the new scheme.

A role whose selectors require none of these labels keeps its permissions after the update, and the alert lists it only for its `rbac.deckhouse.io/kind: manage` and `rbac.deckhouse.io/use-role` labels.
Do not freeze such a role. Run only the commands of the first step, with `BOUND_ROLE` set to the name of the role, so that the explicit RoleBinding objects keep the access in the namespaces of the modules after the update.
The role stays in the alert until it is replaced with a role of the new scheme after the update.

#### Relabeling a capability

Relabel every capability that the `D8UserAuthzLegacyRBACv2CustomCapabilityFound` alert lists.
A capability with the `rbac.deckhouse.io/kind: use` and `rbac.deckhouse.io/aggregate-to-kubernetes-as: <LEVEL>` labels extends the `d8:use:role:<LEVEL>` roles.
In the new scheme, these roles select the `rbac.deckhouse.io/aggregate-to-namespace-as` label. Add it and keep the old labels, which the current roles read:

```shell
d8 k label clusterrole <CAPABILITY_NAME> rbac.deckhouse.io/aggregate-to-namespace-as=<LEVEL>
```

If the capability is applied from a Git repository, add the label to its manifest as well.

After the update, replace the capability with a capability of the new scheme.
Create `d8:custom:namespace-capability:<NAME>` with the same rules and the `rbac.deckhouse.io/kind: custom-capability`, `rbac.deckhouse.io/scope: namespace` and `rbac.deckhouse.io/aggregate-to-namespace-as: <LEVEL>` labels, check the permissions of the subjects, and then delete the old capability.
After the update, the commands are in the "Replacing a capability" section of this FAQ.
Until the capability is replaced, the `rbacv2-cluster-roles.deckhouse.io` webhook refuses every update of it, because it carries aggregation labels without `rbac.deckhouse.io/kind: custom-capability`, and an apply of its manifest from a Git repository fails as well.

#### Moving to a role of the new scheme

Once the built-in capabilities carry the new labels, you can replace a frozen role with a role of the new scheme that aggregates permissions again:

1. Create a role with the `d8:custom:` name prefix, the `rbac.deckhouse.io/kind: custom-role` label, and the new aggregation selectors. See the before and after examples below. Replace a selector by the aggregation label of a lineage that the new role model does not collect as described in [Selectors by a lineage the new role model does not collect](#selectors-by-a-lineage-the-new-role-model-does-not-collect).
1. Recreate your capabilities with the `rbac.deckhouse.io/kind: custom-capability` label and the `d8:custom:` name prefix.
1. For every RoleBinding and ClusterRoleBinding object pointing at the frozen role, create an object with the same subjects pointing at the new role. The `roleRef` field is immutable, so create a new object instead of editing the old one.
1. Check the permissions of the subjects with `d8 k auth can-i --as <USER_NAME>`. Then delete the old RoleBinding and ClusterRoleBinding objects, the frozen role, and the old capabilities. Delete the explicit RoleBinding objects created during the freeze once the new role grants the same access in their namespaces.

#### Selectors by a lineage the new role model does not collect

A selector by the aggregation label of a lineage selects the capabilities that carry this label.
For example, `rbac.deckhouse.io/aggregate-to-virtualization-as: <LEVEL>` selects the capabilities of the `virtualization` module.
In the new role model, the modules label their capabilities with the lineage of the subsystem they are in, and a selector by that lineage selects the capabilities of every module of the subsystem, which can be much more than the role gave before.
For example, the `cluster` subsystem includes most of the modules of the `deckhouse`, `infrastructure` and `kubernetes` subsystems and the `virtualization` module.
To keep the new role as narrow as the old one, select the capabilities themselves by the `rbac.deckhouse.io/capability` label.
Its value does not depend on the subsystem. It is `system-capability.<MODULE_NAME>.<ACTION>` for a system capability and `namespace-capability.<MODULE_NAME>.<ACTION>` for a namespace capability.

1. Once the built-in capabilities carry the new labels, list the capabilities of the module with the levels of the roles that collect them:

   ```shell
   d8 k get clusterroles -l rbac.deckhouse.io/kind=capability,module=<MODULE_NAME> -o json | jq -r '
     .items[]
     | [.metadata.labels["rbac.deckhouse.io/capability"],
        (.metadata.labels | to_entries
          | map(select(.key | test("^rbac\\.deckhouse\\.io/aggregate-to-.+-as$")) | (.key | ltrimstr("rbac.deckhouse.io/aggregate-to-") | rtrimstr("-as")) + "=" + .value)
          | join(","))]
     | @tsv'
   ```

1. Keep the capabilities that the old selector matched. A selector with the `<LEVEL>` value matched the capabilities of this level and, through the built-in roles of the lower levels, the capabilities of those levels as well.
1. In the new role, put one selector by the `rbac.deckhouse.io/capability` label for each of them. To give all the system capabilities of the module whatever their level, a single selector by `rbac.deckhouse.io/scope: system` and `module: <MODULE_NAME>` is enough.

For example, the following namespace role gives in a namespace what a selector by `rbac.deckhouse.io/aggregate-to-virtualization-as: user` gave, without the other modules of the subsystem. Replace `<ACTION>` with the values from the list:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:virtualization-user
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
rules: []
```

A role of the system or subsystem scope selects the `system-capability.<MODULE_NAME>.<ACTION>` values the same way.

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
  aggregationRule:
    clusterRoleSelectors:
      - matchLabels:
          rbac.deckhouse.io/aggregate-to-cluster-as: manager
      - matchLabels:
          rbac.deckhouse.io/scope: system
          module: user-authn
  rules: []
  ```

What changed:

- The name got the mandatory `d8:custom:` prefix and names the subsystem of the role
- `rbac.deckhouse.io/kind: manage` → `rbac.deckhouse.io/kind: custom-role`
- `rbac.deckhouse.io/level: subsystem` → `rbac.deckhouse.io/scope: subsystem`
- The `rbac.deckhouse.io/aggregate-to-all-as` label is removed. A custom role cannot be aggregated into `d8:system:<level>`, because only a custom capability may carry the `rbac.deckhouse.io/aggregate-to-system-as` label. To give the role to the subjects of `d8:system:manager`, bind it to them with a ClusterRoleBinding
- The `rbac.deckhouse.io/kind: manage` label is removed from the aggregation selectors
- All system permissions of a module are now selected with `rbac.deckhouse.io/scope: system` + `module: <module>`
- The `deckhouse` and `kubernetes` subsystems are part of the `cluster` subsystem in the new role model, so their two selectors became one. It selects the whole `cluster` subsystem, which is wider than the two old ones. To keep the role as narrow as before, select the capabilities of the modules you need by `rbac.deckhouse.io/capability` instead, see [Selectors by a lineage the new role model does not collect](#selectors-by-a-lineage-the-new-role-model-does-not-collect)

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
| `rbac.deckhouse.io/aggregate-to-all-as` | `<level>` | Renamed to `rbac.deckhouse.io/aggregate-to-system-as` on a capability, removed from a custom role | Aggregates the object into the system-wide role (`d8:system:<level>`) |
| `rbac.deckhouse.io/aggregate-to-<subsystem>-as` | Used in selectors together with `rbac.deckhouse.io/kind: manage` | Used in selectors on its own | Aggregates the object into the subsystem role of the given level |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as` | `<level>` (for use permissions) | Renamed to `rbac.deckhouse.io/aggregate-to-namespace-as` | Aggregates the object into the namespace role (`d8:namespace:<level>`) |
| `rbac.deckhouse.io/namespace` | Namespace | Unchanged | An additional namespace where a RoleBinding is automatically created for the role holders |
| `rbac.deckhouse.io/capability` | — | A unique capability name (for example, `system-capability.deckhouse.view`) | A machine-readable identifier of a built-in capability |
| `rbac.deckhouse.io/deprecated` | — | `"true"` on alias roles | The role is deprecated and will be removed; migrate the bindings to the new role |
| `module` | Module name | Unchanged | Marks a built-in object as belonging to a DP module; handy in aggregation selectors together with `scope` |
| `heritage: deckhouse` | Platform object marker | Unchanged | Must not be set on custom objects |

Annotations on ClusterRole objects (the old scheme did not use annotations):

| Annotation | Purpose |
|------------|---------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | The displayed name and description of a role/capability in Russian (the platform sets them on built-in objects; you can set your own on custom ones) |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | Same in English |
| `rbac.deckhouse.io/deprecated-replaced-by` | Comes with the new scheme and is set on every role that keeps its previous name for compatibility. It contains the name of the new role to migrate the bindings to. The previous role aggregates the same capabilities as that new role, so it grants the permissions of the new role, which can differ from the permissions it granted before. The previous names are temporary |

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
