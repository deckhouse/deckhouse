# Migrating a module to the RBACv2 role model of DKP 1.78

Audience: teams that own a DKP module shipping `templates/rbacv2/**`. The work happens in the module
repository — nothing here touches a cluster. Every module in the platform repository has already been
migrated; this document, the script next to it, and the [agent prompt](./RBACV2_MODULE_MIGRATION_PROMPT.md)
are for modules that live elsewhere.

The user-facing side of the change — role names, custom roles, bindings — is in the module
documentation ([README](../README.md), [FAQ](../FAQ.md)). Here we only deal with the objects a module
ships itself.

## TL;DR

```shell
# In the module repository:
curl -sO https://raw.githubusercontent.com/deckhouse/deckhouse/main/modules/140-user-authz/docs/internal/rbacv2-migrate-module.sh
chmod +x rbacv2-migrate-module.sh
./rbacv2-migrate-module.sh -n     # look at the diff
./rbacv2-migrate-module.sh        # apply it
```

Then read [what to check by hand](#what-the-script-cannot-decide) — the script rewrites labels and
names correctly, but it cannot tell whether a permission belongs where it currently sits.

## What changed in the model

Permissions live in **capabilities**: `ClusterRole` objects that carry rules and one or more
`rbac.deckhouse.io/aggregate-to-<lineage>-as: <level>` labels. **Roles** are empty `ClusterRole`
objects whose `aggregationRule` selects those labels; Kubernetes fills them in. A module ships
capabilities only — the roles belong to `user-authz` and `multitenancy-manager`.

| Lineage | Levels | Granted with | What a module puts there |
|---------|--------|--------------|--------------------------|
| `namespace` | `viewer`, `user`, `manager`, `admin`, `superadmin` | `RoleBinding` in a namespace | The module's namespaced resources — what a tenant works with |
| `<subsystem>`: `deckhouse`, `infrastructure`, `kubernetes`, `networking`, `observability`, `security`, `storage` | `viewer`, `manager`, `superadmin` | `ClusterRoleBinding` | The module's cluster-scoped resources and its `ModuleConfig` |
| `system` | `viewer`, `manager`, `superadmin` | `ClusterRoleBinding` | Nothing directly: the system roles aggregate the subsystem ones |
| `project` | `viewer`, `user`, `manager`, `admin`, `superadmin` | `ProjectRoleBinding` | Project-structure resources (multitenancy-manager only) |

The legacy scheme had the same two tiers under different names: `use` for what happens inside a
namespace, `manage` for the module's own configuration. The migration keeps that split — it renames
the objects, replaces the labels the aggregation is driven by, and moves the namespace tier off the
`kubernetes` subsystem onto the `namespace` lineage where it belongs.

## The mapping

| Legacy | New |
|--------|-----|
| `d8:use:capability:module:<module>:<view\|edit>` | `d8:namespace-capability:<module>:<view\|edit>` |
| `d8:manage:permission:module:<module>:<view\|edit>` | `d8:system-capability:<module>:<view\|edit>` |

| Legacy label | New label |
|--------------|-----------|
| `rbac.deckhouse.io/kind: use` or `manage` | `rbac.deckhouse.io/kind: capability` |
| — | `rbac.deckhouse.io/scope: namespace` (was `use`) or `system` (was `manage`) |
| — | `rbac.deckhouse.io/capability: "<scope>-capability.<module>.<action>"` — globally unique, this is what lets a custom role or the console pick out exactly this capability |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as: <level>` on a `use` object (and any second lineage next to it) | a single `rbac.deckhouse.io/aggregate-to-namespace-as: <level>`, same level |
| `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` on a `manage` object | unchanged, including several at once |
| `rbac.deckhouse.io/level: module` | removed, nothing reads it |
| `rbac.deckhouse.io/namespace: d8-<module>` | unchanged — but it is only read on objects scoped `system` or `subsystem` |

Localized titles and descriptions are now mandatory on every object, and their wording is
conventional across the platform (the script generates exactly these):

```yaml
  annotations:
    en.meta.deckhouse.io/title: "Module <module>: view"
    ru.meta.deckhouse.io/title: "Модуль <module>: просмотр"
    en.meta.deckhouse.io/description: "Read-only access to <module> resources in a namespace."
    ru.meta.deckhouse.io/description: "Доступ только на чтение к ресурсам модуля <module> в пространстве имён."
```

## A full example

Before — the module's namespaced permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/aggregate-to-kubernetes-as: viewer
    rbac.deckhouse.io/kind: use
  name: d8:use:capability:module:mymodule:view
rules:
  - apiGroups: ["mygroup.io"]
    resources: ["myresources"]
    verbs: ["get", "list", "watch"]
```

After:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/aggregate-to-namespace-as: viewer
    rbac.deckhouse.io/kind: capability
    rbac.deckhouse.io/capability: "namespace-capability.mymodule.view"
    rbac.deckhouse.io/scope: namespace
  name: d8:namespace-capability:mymodule:view
  annotations:
    en.meta.deckhouse.io/title: "Module mymodule: view"
    ru.meta.deckhouse.io/title: "Модуль mymodule: просмотр"
    en.meta.deckhouse.io/description: "Read-only access to mymodule resources in a namespace."
    ru.meta.deckhouse.io/description: "Доступ только на чтение к ресурсам модуля mymodule в пространстве имён."
rules:
  - apiGroups: ["mygroup.io"]
    resources: ["myresources"]
    verbs: ["get", "list", "watch"]
```

And the module's configuration, which stays in the subsystem lineage it already used:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/aggregate-to-networking-as: manager
    rbac.deckhouse.io/kind: capability
    rbac.deckhouse.io/capability: "system-capability.mymodule.edit"
    rbac.deckhouse.io/scope: system
    rbac.deckhouse.io/namespace: d8-mymodule
  name: d8:system-capability:mymodule:edit
  annotations:
    en.meta.deckhouse.io/title: "Module mymodule: edit configuration"
    ru.meta.deckhouse.io/title: "Модуль mymodule: управление конфигурацией"
    en.meta.deckhouse.io/description: "Manage the mymodule module configuration."
    ru.meta.deckhouse.io/description: "Управление конфигурацией модуля mymodule."
rules:
  - apiGroups: ["deckhouse.io"]
    resources: ["moduleconfigs"]
    resourceNames: ["mymodule"]
    verbs: ["create", "update", "patch", "delete"]
```

A module whose labels are templated through `helm_lib_module_labels` keeps them inside the `dict`
call; the same pairs change, in the same way.

## The script

`rbacv2-migrate-module.sh` walks `**/templates/rbacv2/**`, rewrites every legacy module capability it
finds, and reports what it did not dare to touch. It replays the platform migration byte for byte on
93 of the 95 in-tree files (the remaining two also had their rules edited by hand, which is a
decision, not a rewrite), so the mechanical part can be trusted.

```shell
./rbacv2-migrate-module.sh -n path/to/module   # diff only
./rbacv2-migrate-module.sh path/to/module      # rewrite in place
```

It is idempotent: an already migrated file is reported as untouched. It never renames files, so the
diff stays reviewable.

### What the script cannot decide

1. **Whether a permission sits in the right tier.** A `use` capability now aggregates into namespace
   roles, and rules on cluster-scoped resources grant nothing there — they need a system capability.
   The legacy scheme was sloppier about this because `use` capabilities aggregated into a cluster-wide
   subsystem role, where a cluster-scoped rule did work. Check every rule of a migrated `use` file:
   if the resource is cluster-scoped, move it to `manage/`.
2. **The level.** `view` → `viewer` and `edit` → `manager` is what almost every module used, and the
   script keeps whatever it finds. The namespace lineage also has `user` (a tenant who works in the
   namespace) and `admin` (who manages it). If your `edit` capability is really something a plain
   tenant should have, `user` is the level for it.
3. **Two lineages at once.** A legacy `use` capability sometimes carried both `kubernetes` and
   `networking`; the second one is dropped, because a namespaced capability has nothing to do in a
   subsystem lineage. If the module genuinely also wants its cluster-scoped part in `networking`, that
   part belongs in a separate system capability.
4. **The wording.** The generated titles and descriptions follow the platform convention and mention
   the module by name; read them once — they are what the console shows next to the capability.

## Checking the result

In the repository:

```shell
# The templates still render.
helm template . --set global.enabledModules='{}' >/dev/null

# Nothing legacy is left.
grep -rn 'rbac.deckhouse.io/kind: \(use\|manage\)\|d8:use:capability\|d8:manage:permission\|rbac.deckhouse.io/level' templates/
```

The contract the platform enforces on its own modules is in
[`testing/rbacv2`](../../../../testing/rbacv2) of the `deckhouse` repository (`go test -tags validation ./testing/rbacv2/`):
the `d8:` name prefix and the naming pattern per scope, the presence of `kind` and `scope`, the four
i18n annotations, a role with an `aggregationRule` and no rules of its own versus a capability with
rules and no `aggregationRule`, a unique `rbac.deckhouse.io/capability` marker and at least one
aggregation label on every capability, and `rbac.deckhouse.io/delegatable` only on namespace and
project roles.

In a cluster with the module installed:

```shell
# The namespaced permissions arrived in the namespace roles.
d8 k get clusterrole d8:namespace:viewer -o json | jq '[.rules[] | select(.apiGroups[] == "mygroup.io")]'

# The configuration permissions arrived in the subsystem role.
d8 k get clusterrole d8:subsystem:networking:manager -o json | jq '[.rules[] | select(.resourceNames[]? == "mymodule")]'
```

## What happens if a module is not migrated

Nothing rejects the old objects and nothing is logged — the labels are simply read by different code
now. Three consequences, all confirmed against a running cluster:

- **A `use` capability is granted cluster-wide instead of per namespace.** `aggregate-to-kubernetes-as`
  now belongs to the cluster-scoped `kubernetes` subsystem, so the module's namespaced rules land in
  `d8:subsystem:kubernetes:*` and, up the ladder, in `d8:system:*` — every holder of those roles gets
  them across the whole cluster. `d8:namespace:*` no longer contains them at all, so tenants lose the
  access they had.
- **A `use` capability at the `user` or `admin` level disappears.** The subsystem lineages have no such
  levels, nothing selects the object, and its rules go nowhere.
- **The automatic `RoleBinding` in the module's namespace stops being created.** The hook that fans a
  `ClusterRoleBinding` on a system or subsystem role out into namespaced `RoleBinding` objects only
  looks at objects labelled `rbac.deckhouse.io/scope: system` or `subsystem`, so without that label the
  `rbac.deckhouse.io/namespace` label is never read.

A `manage` capability keeps aggregating into the same subsystem role, so its permissions do not move —
but it stays invisible to the console and to the permission browser, which classify objects by
`rbac.deckhouse.io/kind` and `rbac.deckhouse.io/scope`.
