# Migrating a module's RBACv2 templates to the role model of DKP 1.78

Audience: authors of DKP modules that ship `templates/rbacv2/**`. Every in-tree module has already
been migrated; this document is for modules that live outside this repository, and for reviewing what
the migration actually changes.

The user-facing side of the change — role names, custom roles, and bindings — is described in the
module documentation ([README](../README.md), [FAQ](../FAQ.md)). This document covers only the
objects a module ships itself.

## What the model looks like now

Permissions live in **capabilities** — `ClusterRole` objects that carry rules and a set of
`rbac.deckhouse.io/aggregate-to-<lineage>-as: <level>` labels. **Roles** are empty `ClusterRole`
objects whose `aggregationRule` selects those labels. A module ships capabilities only; the roles
belong to `user-authz` and `multitenancy-manager`.

There are four lineages a module can aggregate into:

| Lineage | Levels | Granted in | Typical module content |
|---------|--------|------------|------------------------|
| `namespace` | `viewer`, `user`, `manager`, `admin`, `superadmin` | A namespace, via `RoleBinding` | The module's namespaced custom resources |
| `project` | the same | A project, via `ProjectRoleBinding` | Project-structure resources (multitenancy only) |
| `<subsystem>` (`deckhouse`, `infrastructure`, `kubernetes`, `networking`, `observability`, `security`, `storage`) | `viewer`, `manager`, `superadmin` | The cluster, via `ClusterRoleBinding` | The module's cluster-scoped resources and its `ModuleConfig` |
| `system` | `viewer`, `manager`, `superadmin` | The cluster, via `ClusterRoleBinding` | Nothing directly: the system roles aggregate the subsystem ones |

A module usually ships two pairs of capabilities: `use/{view,edit}.yaml` for what a tenant does
inside a namespace, and `manage/{view,edit}.yaml` for the module's own configuration.

## The mapping

| Legacy object | New object |
|---------------|------------|
| `d8:use:capability:module:<module>:<view\|edit>` | `d8:namespace-capability:<module>:<view\|edit>` |
| `d8:manage:permission:module:<module>:<view\|edit>` | `d8:system-capability:<module>:<view\|edit>` |

| Legacy label | New label |
|--------------|-----------|
| `rbac.deckhouse.io/kind: use` or `manage` | `rbac.deckhouse.io/kind: capability` |
| — | `rbac.deckhouse.io/scope: namespace` (was `use`) or `system` (was `manage`) |
| — | `rbac.deckhouse.io/capability: "<scope>-capability.<module>.<name>"`, globally unique |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as: <level>` on a `use` capability | `rbac.deckhouse.io/aggregate-to-namespace-as: <level>` |
| `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` on a `manage` capability | unchanged |
| `rbac.deckhouse.io/level: module` | removed |
| `rbac.deckhouse.io/namespace: d8-<module>` | unchanged, but only read when `scope` is `system` or `subsystem` |

Localized titles and descriptions are now mandatory:

```yaml
  annotations:
    en.meta.deckhouse.io/title: "Module <module>: view"
    ru.meta.deckhouse.io/title: "Модуль <module>: просмотр"
    en.meta.deckhouse.io/description: "Read-only access to <module> resources in a namespace."
    ru.meta.deckhouse.io/description: "Доступ только на чтение к ресурсам модуля <module> в пространстве имён."
```

## A full example

Before — the module's namespaced permissions in the legacy scheme:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:use:capability:module:mymodule:view
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: viewer
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
  name: d8:namespace-capability:mymodule:view
  annotations:
    en.meta.deckhouse.io/title: "Module mymodule: view"
    ru.meta.deckhouse.io/title: "Модуль mymodule: просмотр"
    en.meta.deckhouse.io/description: "Read-only access to mymodule resources in a namespace."
    ru.meta.deckhouse.io/description: "Доступ только на чтение к ресурсам модуля mymodule в пространстве имён."
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/kind: capability
    rbac.deckhouse.io/capability: "namespace-capability.mymodule.view"
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/aggregate-to-namespace-as: viewer
rules:
  - apiGroups: ["mygroup.io"]
    resources: ["myresources"]
    verbs: ["get", "list", "watch"]
```

And the module's own configuration, which stays in the subsystem lineage it already used:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:system-capability:mymodule:edit
  annotations:
    en.meta.deckhouse.io/title: "Module mymodule: manage configuration"
    ru.meta.deckhouse.io/title: "Модуль mymodule: управление конфигурацией"
    en.meta.deckhouse.io/description: "Management of the mymodule module configuration."
    ru.meta.deckhouse.io/description: "Управление конфигурацией модуля mymodule."
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/kind: capability
    rbac.deckhouse.io/capability: "system-capability.mymodule.edit"
    rbac.deckhouse.io/scope: system
    rbac.deckhouse.io/namespace: d8-mymodule
    rbac.deckhouse.io/aggregate-to-networking-as: manager
rules:
  - apiGroups: ["deckhouse.io"]
    resources: ["moduleconfigs"]
    resourceNames: ["mymodule"]
    verbs: ["create", "update", "patch", "delete"]
```

## What happens if a module is not migrated

Nothing rejects the old objects, and no error is logged — the labels are simply read by different
code now. Three separate consequences, all confirmed against a running cluster:

- **A `use` capability is granted cluster-wide instead of per namespace.** `aggregate-to-kubernetes-as`
  now belongs to the cluster-scoped `kubernetes` subsystem, so the module's namespaced rules end up in
  `d8:subsystem:kubernetes:viewer` and, through the ladder, in `d8:system:*` — handed to every holder
  of those roles across the whole cluster. Meanwhile `d8:namespace:*` no longer contains them at all,
  so tenants lose the access they used to have.
- **A `use` capability at the `user` or `admin` level disappears.** The subsystem lineages have no such
  levels, so nothing selects the object and its rules go nowhere.
- **The automatic `RoleBinding` in the module's namespace stops being created.** The hook that fans a
  `ClusterRoleBinding` on a system/subsystem role out into namespaced `RoleBinding` objects only looks
  at objects labelled `rbac.deckhouse.io/scope: system` or `subsystem`. Without the `scope` label the
  `rbac.deckhouse.io/namespace` label is never read.

A `manage` capability keeps aggregating into the same subsystem role as before, so its permissions do
not move — but it stays invisible to the console and to the permission browser, which classify objects
by `rbac.deckhouse.io/kind` and `scope`.

## Checking the result

In-tree modules are checked by the template contract test, which is the authoritative list of
requirements:

```shell
go test -tags validation ./testing/rbacv2/
```

It enforces the `d8:` name prefix, the naming pattern per scope, the presence of `kind` and `scope`,
the i18n annotations, that a role has an `aggregationRule` and no rules of its own while a capability
has rules and no `aggregationRule`, that every capability carries a unique
`rbac.deckhouse.io/capability` marker and at least one aggregation label, and that
`rbac.deckhouse.io/delegatable` appears only on namespace and project roles.

An out-of-tree module can check itself in a cluster: render the templates and confirm the rules
arrive where they are supposed to.

```shell
d8 k get clusterrole d8:namespace:viewer -o json | jq '[.rules[] | select(.apiGroups[] == "mygroup.io")]'
d8 k get clusterrole d8:subsystem:networking:manager -o json | jq '[.rules[] | select(.apiGroups[] == "deckhouse.io")]'
```
