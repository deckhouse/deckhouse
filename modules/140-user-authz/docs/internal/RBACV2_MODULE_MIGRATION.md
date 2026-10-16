# Migrating a module to the RBACv2 role model of DKP 1.78

Audience: teams that own a DKP module shipping `templates/rbacv2/**`. The work happens in the module
repository — nothing here touches a cluster. Every module in the platform repository has already been
migrated; this document, the script next to it, and the [agent prompt](./RBACV2_MODULE_MIGRATION_PROMPT.md)
are for modules that live elsewhere.

The user-facing side of the change — role names, custom roles, bindings — is in the module
documentation ([README](../README.md), [FAQ](../FAQ.md)). Here we only deal with the objects a module
ships itself.

## TL;DR

Set `subsystems` in `module.yaml` to the module's subsystem first ([The subsystems](#the-subsystems)).

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

A role manifest carries no `rules` key at all — not even `rules: []`. The field belongs to the
`clusterrole-aggregation-controller`, and a chart that declares it fights the controller for it:
a server-side apply then fails with `conflict with "clusterrole-aggregation-controller": .rules`,
and forcing the conflict rewrites the object twice on every reconcile, leaving the role empty for
the moment between the two writes.

| Lineage | Levels | Granted with | What a module puts there |
|---------|--------|--------------|--------------------------|
| `namespace` | `viewer`, `user`, `manager`, `admin`, `superadmin` | `RoleBinding` in a namespace | The module's namespaced resources — what a tenant works with |
| `<subsystem>`, one of the [eight the platform ships](#the-subsystems) or [one of the module's own](#a-subsystem-of-the-modules-own) | `viewer`, `manager`, `superadmin` | `ClusterRoleBinding` | The module's cluster-scoped resources and its `ModuleConfig` |
| `system` | `viewer`, `manager`, `superadmin` | `ClusterRoleBinding` | Nothing directly: the system roles aggregate the subsystem ones |
| `project` | `viewer`, `user`, `manager`, `admin`, `superadmin` | `ProjectRoleBinding` | Project-structure resources (multitenancy-manager only) |

The legacy scheme had the same two tiers under different names: `use` for what happens inside a
namespace, `manage` for the module's own configuration. The migration keeps that split — it renames
the objects, replaces the labels the aggregation is driven by, and moves the namespace tier off the
`kubernetes` subsystem onto the `namespace` lineage where it belongs.

### The subsystems

The role model has eight subsystems. They are `iam`, `security`, `cluster`, `delivery`, `network`,
`storage`, `observability` and `managed-services`. The roles
`d8:subsystem:<subsystem>:{viewer,manager,superadmin}` of each of them come from user-authz, so a
module ships no role of a platform subsystem.

The DKP subsystems table, an internal document, puts every DKP module in one of them. Set
`subsystems` in `module.yaml` to the module's subsystem from the table before running the script,
since the script reads it.

The legacy scheme had seven subsystems. Each of them went to a subsystem of the role model.

| Legacy subsystem | Subsystem of the role model |
|------------------|-----------------------------|
| `deckhouse`, `infrastructure`, `kubernetes` | `cluster` |
| `networking` | `network` |
| `observability`, `security`, `storage` | the same |
| a subsystem of the module's own (`virtualization`) | the one the table gives the module (`cluster` for virtualization) |

`iam`, `delivery` and `managed-services` are new. No role of the model aggregates a legacy lineage, and
there is no period in which a capability carries both labels. A capability left on a legacy lineage
reaches no role, so the script moves every subsystem lineage of a system capability. When `module.yaml`
names one subsystem of the role model, every such lineage goes to it, whichever subsystem it named
before. That is how the script makes the moves of the DKP subsystems table, such as local-path-provisioner
from `infrastructure` to `storage` and user-authz from `security` to `iam`. When `module.yaml` names
none of them or several, the script moves the legacy lineages by the table above and reports a lineage
that `module.yaml` does not declare. An `rbac.yaml` that lists `subsystems` takes the place of
`module.yaml` here, as it does for dmt.

### A subsystem of the module's own

The eight subsystems cover the platform. A module that the DKP subsystems table puts in none of them
may keep a subsystem of its own. The table puts every DKP module in one of the eight now, so this is
left for a module from outside DKP, and the webhook and the `contract` rule of `dmt` keep protecting
such a subsystem as long as the role model allows it.

- `module.yaml` declares it in `subsystems` (`subsystems: [<name>]`).
- The module ships the subsystem roles `d8:subsystem:<name>:{viewer,manager,superadmin}` itself, shaped
  as user-authz's own (`templates/rbacv2/global/subsystem/roles/cluster/`):

  ```yaml
  metadata:
    name: d8:subsystem:<name>:manager
    annotations: {en|ru.meta.deckhouse.io/title|description: ...}
    labels:
      rbac.deckhouse.io/kind: role
      rbac.deckhouse.io/scope: subsystem
      rbac.deckhouse.io/subsystem: <name>
      rbac.deckhouse.io/use-role: admin                      # viewer: viewer, superadmin: superadmin
      rbac.deckhouse.io/aggregate-to-<name>-as: superadmin   # the ladder; viewer: manager, superadmin: none
      rbac.deckhouse.io/aggregate-to-system-as: manager      # into d8:system:<level>
  aggregationRule:
    clusterRoleSelectors:
      - matchLabels: {rbac.deckhouse.io/aggregate-to-<name>-as: manager}
        # a use capability of the previous scheme gives namespace rights, a cluster-wide role leaves it out
        matchExpressions: [{key: rbac.deckhouse.io/kind, operator: NotIn, values: [use]}]
  ```

- Its system capabilities `d8:system-capability:<module>:<action>` aggregate with
  `rbac.deckhouse.io/aggregate-to-<name>-as`, as into any subsystem.

The `rbacv2-cluster-roles` webhook reads the subsystem roles of the cluster, so the lineage of such a
subsystem is as protected as the platform's. An object that is not a capability of the model cannot
carry its aggregation label, and a role granted in a namespace or a project cannot aggregate it. The
`contract` rule of `dmt` accepts a subsystem the module's `module.yaml` declares. Another module
cannot aggregate into it, since to the linter it is that module's own.

A module that the table puts in a platform subsystem ships no subsystem role, not even for a while.
Two Helm releases cannot own one object, so a `d8:subsystem:cluster:*` role in a module's chart
collides with user-authz's. The virtualization module had a subsystem of its own in the legacy scheme
and is in `cluster` now. Its system capabilities aggregate with
`rbac.deckhouse.io/aggregate-to-cluster-as`, and `d8:subsystem:cluster:*` comes from user-authz.

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
| `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` on a `manage` object | the same label of the [subsystem of the role model](#the-subsystems) that took the subsystem over, same level, several at once kept; two that land on one subsystem are merged |
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

And the module's configuration. Its lineage moves to the subsystem of the role model that took over the
legacy one, from `networking` to `network`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/aggregate-to-network-as: manager
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
decision, not a rewrite), so the mechanical part can be trusted. The subsystem lineages are the one
difference, since the script moves them to the subsystems of the role model.

```shell
./rbacv2-migrate-module.sh -n path/to/module          # diff only
./rbacv2-migrate-module.sh path/to/module             # keep both models, gated by version
./rbacv2-migrate-module.sh --replace path/to/module   # new model only
```

It is idempotent: an already migrated file is reported as untouched. A file migrated before the
subsystems of the role model were settled gets its subsystem lineages moved, and in a gated file only
the new branch changes. If an earlier run wrote `d8:subsystem:<module>:<level>` for a module that
`module.yaml` now puts in a platform subsystem, the script reports the role for you to drop, and in a
gated file that means the new branch only. It also names the alias that the released name
`d8:manage:<module>:<level>` needs ([item 7](#what-the-script-cannot-decide)). It never renames files,
so the diff stays reviewable.

### Serving both models from one branch

An external module is installed on clusters of more than one platform version, and the two models do
not see each other: an object of one is invisible to the other. So by default the script keeps the
legacy object, puts the migrated one beside it, and wraps both:

```text
{{- if eq (include "<module>.rbacv2_new_scheme" .) "true" }}
<the migrated object>
{{- else }}
<the legacy object, unchanged>
{{- end }}
```

The gate itself lands in `templates/_rbacv2_compat.tpl`, written once per module. It reads
`global.deckhouseVersion` and answers `true` from DKP 1.78 on. That value is the content of
`/deckhouse/version`, put into the global values by a startup hook, and it reaches an external
module's render like any other global value — `upmeter` already builds its user-agent out of it.
A module delivered as a `ModulePackage` gets it in the same place; only an `Application` package
reads global values as `.Platform.<path>` instead, and RBACv2 capabilities are not shipped that way.

Six details in the gate matter:

- that value is `dev` on a development build and `unknown` when the version file is missing, and
  `semverCompare` raises on both — a raise takes down the render of the whole module, so anything
  that is not a version has to answer without comparing;
- it answers `true`, for the direction of failure rather than the likelier branch. The version file
  holds `CI_COMMIT_TAG`, so a build off any branch says `dev`, and a dev stand built from
  `release-1.77` looks exactly like one built from `main`. Only one of the two mistakes is dangerous:
  the new object on a cluster of the legacy scheme is inert, since nothing there selects
  `aggregate-to-namespace-as`, and the module's permissions are merely missing; a legacy use capability
  on a cluster of the role model that carries the lineage of a subsystem the role model kept (`observability`, `security`, `storage`) is
  aggregated by `d8:subsystem:<subsystem>:<level>`, which is granted through a `ClusterRoleBinding`,
  so rules meant for one namespace become cluster-wide. A dev stand on a release branch below 1.78
  that needs the legacy objects flips the `true` in the helper to `false` in that build;
- the comparison is on major and minor only, because semver orders `1.78.0-rc.1` below `1.78.0` and a
  release candidate of 1.78 would otherwise be served the legacy object;
- the regex uses `[.]`, since a backslash escape inside a Go template string literal is a parse error;
- the version is piped through `toString`, because `deckhouseVersion: 1.78` unquoted in a values file
  is a YAML float and `regexFind` on a non-string kills the render;
- `.Values.global` is parenthesized, so a chart rendered with no global values at all falls back to
  `dev` instead of failing on a nil pointer.

The module's own template tests need attention, because a test that looks up a ClusterRole by name
finds it only in the branch its render selects, and a harness that leaves `global.deckhouseVersion`
unset renders the new scheme. Set the version explicitly in such a test and cover both branches; that
is also the cheapest place to prove the gate works.

In-tree modules must use `--replace`, and not merely prefer it: the platform's own contract test
(`testing/rbacv2/`) reads every file under `templates/rbacv2/**` as raw YAML, and a gated file fails
it — which is also why the in-tree compatibility aliases live in `templates/rbacv2-compat/` instead.
External modules are unaffected: dmt renders the chart through helm and lints the objects, so it only
ever sees one branch.

A module whose `module.yaml` already requires 1.78 or newer does not need the legacy branch:
`--replace` writes the new object alone, which is also what in-tree modules use, since they ship with
the platform they target. Remove the gate and the `else` branch once the module drops support for
clusters below 1.78.

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
   subsystem lineage. If the module genuinely also wants its cluster-scoped part in `network`, that
   part belongs in a separate system capability.
4. **The wording.** The generated titles and descriptions follow the platform convention and mention
   the module by name; read them once — they are what the console shows next to the capability.
5. **An action of the module's own.** A capability named after something other than `view` or `edit`
   (`d8:use:capability:module:state-snapshotter:download_snapshots`) keeps its action:
   `d8:namespace-capability:state-snapshotter:download_snapshots`. There is no convention for its texts, so
   the script writes them from the action's name and says so; reword them in both languages.
6. **An axis of the module, and a role that collects it.** A label `rbac.deckhouse.io/aggregate-to-<axis>-as`
   whose axis is not a lineage of the platform (state-snapshotter's `backup-agent`) is kept, so the role
   that collects the capability still has it, and reported: the 1.78 contract refuses it as an unknown
   lineage. A capability carrying only such an axis is left alone — it aggregates into no level of the
   platform, and the contract refuses that too; pick a level for it, then rerun. A legacy name on a role
   that only aggregates (an `aggregationRule` and no rules) is not a capability, and the script leaves it
   alone. The 1.78 model has no role of a module: ship it as an ordinary ClusterRole outside the model —
   without `rbac.deckhouse.io/kind`, named `d8:<module>:<name>`, with an `aggregationRule` by a label of
   the module outside `rbac.deckhouse.io/` (`state-snapshotter.deckhouse.io/aggregate-to-backup-agent: "true"`),
   the label put on every capability it collects, in every module that ships one. Keep the old name
   as an alias in `templates/rbacv2-compat/`, the way user-authz keeps its own: an empty ClusterRole whose
   `aggregationRule` repeats the new role's selector, with the `rbac.deckhouse.io/deprecated: "true"` label
   and the `rbac.deckhouse.io/deprecated-replaced-by` annotation. A binding to the alias raises
   `D8UserAuthzDeprecatedRBACv2ModuleRoleInUse` and does not hold a Deckhouse release. The annotation
   is required: an alias without it is ignored with a warning in the log. The same works for a later
   rename of a `d8:<module>:<name>` role: the alias may carry any `d8:` name except the platform's own
   role families (`d8:system:`, `d8:subsystem:`, `d8:namespace:`, `d8:project:`, `d8:custom:`, `d8:dict`).
7. **A subsystem of the module's own.** A module that had one in the legacy scheme (`subsystems: [virtualization]`)
   named its objects after it: `d8:use:capability:<module>:<action>`, `d8:manage:permission:subsystem:<module>:<action>`,
   namespace roles `d8:use:role:<level>:<module>` and subsystem roles `d8:manage:<subsystem>:<level>`, all on the
   lineage of the subsystem, told apart by `rbac.deckhouse.io/kind`. Where they go depends on `module.yaml`. While it
   declares the module's own subsystem, the subsystem stays the module's. Once it declares one subsystem of the
   platform instead (`subsystems: [cluster]`), the module moves into that one. The script migrates them:
   - a use capability becomes a namespace capability, its level moves from the subsystem's lineage to
     `aggregate-to-namespace-as`, and that label goes: the 1.78 subsystem roles select by it alone, and a namespace
     capability that kept it would pour its rules into the cluster-wide role;
   - a manage permission becomes a system capability on the lineage of the module's own subsystem, or of the platform
     subsystem the module moved into;
   - `d8:manage:<subsystem>:<level>` becomes the subsystem role `d8:subsystem:<subsystem>:<level>` (kind `role`, scope
     `subsystem`, `aggregate-to-all-as` moved to `aggregate-to-system-as`, the selector by the lineage alone) while the
     subsystem stays the module's. Once the module moves into a platform subsystem, the role has no counterpart, since
     user-authz ships the roles of that subsystem, and it stays in the legacy branch of the gate only;
   - a namespace role of the module has no counterpart — `d8:namespace:<level>` gathers the capabilities — so it stays
     in the legacy branch of the gate only, and `--replace` leaves it for you to delete.

   Keep the name of a role that has no counterpart as an alias in `templates/rbacv2-compat/` if bindings use it, as
   in item 6. The alias grants no more than the role did. It selects the capabilities of the module by its `module`
   label, `rbac.deckhouse.io/kind: capability` and the lineage label at each level up to its own. The script names
   the selectors in its report. The rest depends on where the role is granted.
   - The alias of a namespace role, `d8:use:role:<level>:<module>` or `d8:use:<module>:superadmin`, carries the
     `rbac.deckhouse.io/delegatable: "true"` label and the `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"`
     annotation, as the `d8:use:role:*` aliases of user-authz do. A RoleBinding to it inside a project namespace keeps
     working, and a new ProjectRoleBinding or ClusterProjectRoleBinding to it is refused.
   - The alias of a subsystem role, `d8:manage:<module>:<level>`, carries neither. It is granted with a
     ClusterRoleBinding and never reaches a project.

   In a chart that keeps the legacy branch, wrap the alias in the gate,
   `{{- if eq (include "<module>.rbacv2_new_scheme" .) "true" }}`, since below the version the gate checks
   the legacy branch still renders the role under the same name.

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: d8:use:role:user:mymodule
     annotations:
       en.meta.deckhouse.io/title: "Deprecated: d8:use:role:user:mymodule"   # and the other three
       rbac.deckhouse.io/deprecated-replaced-by: d8:namespace:user
       rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"
     labels:
       heritage: deckhouse
       module: mymodule
       rbac.deckhouse.io/deprecated: "true"
       rbac.deckhouse.io/delegatable: "true"
   aggregationRule:
     clusterRoleSelectors:
       - matchLabels: {module: mymodule, rbac.deckhouse.io/kind: capability, rbac.deckhouse.io/aggregate-to-namespace-as: viewer}
       - matchLabels: {module: mymodule, rbac.deckhouse.io/kind: capability, rbac.deckhouse.io/aggregate-to-namespace-as: user}
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: d8:manage:mymodule:viewer
     annotations:
       en.meta.deckhouse.io/title: "Deprecated: d8:manage:mymodule:viewer"   # and the other three
       rbac.deckhouse.io/deprecated-replaced-by: d8:subsystem:cluster:viewer
     labels:
       heritage: deckhouse
       module: mymodule
       rbac.deckhouse.io/deprecated: "true"
   aggregationRule:
     clusterRoleSelectors:
       - matchLabels: {module: mymodule, rbac.deckhouse.io/kind: capability, rbac.deckhouse.io/scope: system, rbac.deckhouse.io/aggregate-to-cluster-as: viewer}
   ```

   The script also reports a marker past 63 characters: the API server refuses such a label, so shorten the action.
   A `subsystem:` name is migrated only for the module's own subsystem, one its `module.yaml` declares beyond the
   platform's or the one named after the module, and a template that writes its labels through `helm_lib_module_labels`
   is taken for the module of its chart (the name in `Chart.yaml`). A use capability the script cannot recognise that
   still carries the subsystem's label is reported for you to migrate by hand.

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
i18n annotations, a role with an `aggregationRule` and no rules of its own (no `rules` key at all)
versus a capability with rules and no `aggregationRule`, a unique `rbac.deckhouse.io/capability`
marker and at least one
aggregation label on every capability, and `rbac.deckhouse.io/delegatable` only on namespace and
project roles.

In a cluster with the module installed:

```shell
# The namespaced permissions arrived in the namespace roles.
d8 k get clusterrole d8:namespace:viewer -o json | jq '[.rules[] | select(.apiGroups[] == "mygroup.io")]'

# The configuration permissions arrived in the subsystem role.
d8 k get clusterrole d8:subsystem:network:manager -o json | jq '[.rules[] | select(.resourceNames[]? == "mymodule")]'
```

## What the compatibility aliases do not carry

The old role names survive one release as aggregating aliases. A binding to `d8:manage:*` or
`d8:use:role:*` keeps what the old role granted, which can be narrower than the role that replaces it.
`d8:manage:deckhouse:*`, `d8:manage:infrastructure:*` and `d8:manage:kubernetes:*` select only the
capabilities of the modules that were in those subsystems, while `d8:subsystem:cluster:*` covers the
whole `cluster` subsystem. `d8:manage:security:*` also keeps the rights for user-authn and user-authz,
which moved to `iam`. `d8:manage:networking:*` and `d8:manage:storage:*` select the lineage of
`network` and `storage` without keepalived and local-path-provisioner, which joined these subsystems
from `infrastructure`. The aliases of `deckhouse`, `infrastructure`, `kubernetes` and `networking` and
`d8:manage:all:*` also select the old lineage of their subsystem, which a module released on its own
may still carry. The aliases a module keeps for its own roles are narrow as well ([item 7](#what-the-script-cannot-decide)).

A ClusterRoleBinding to a `d8:manage:*` alias keeps the RoleBindings that the previous release created
for it in the namespaces of the modules of the old subsystem, `d8:use:<role>:binding:<name>` to
`d8:use:role:<role>`, and user-authz-controller keeps them in sync. Recreating the binding on the new
name replaces them with RoleBindings to the namespace role of the new role in the namespaces of the
modules of the new subsystem.

What does not come back is `serviceaccounts/token` and `impersonate`, which moved from admin to
superadmin by design.

Check your own documentation as well: examples that name `d8:manage:*` or `d8:use:role:*` keep
working only for this release. The `d8:use:role:*` aliases are delegatable, so a plain RoleBinding to
one of them inside a project namespace keeps working; what is refused is a new ProjectRoleBinding or
ClusterProjectRoleBinding to an old name, because the alias carries
`rbac.deckhouse.io/disabled-for-direct-use-in-projects`. The aliases a module keeps for its own namespace roles
carry the same label and annotation (see [item 7](#what-the-script-cannot-decide)).

## Before upgrading: roles that carry a platform aggregation label

A ClusterRole labelled `rbac.deckhouse.io/aggregate-to-<lineage>-as` is merged into the platform role
of that lineage, and the aggregation controller reads nothing but the label. The
`rbacv2-cluster-roles.deckhouse.io` webhook now admits that label only on a `custom-capability`. A
webhook sees writes: a role that already carries the label without being a custom capability keeps
aggregating after the upgrade, and its next `UPDATE` — a GitOps re-apply included — is refused. Such
roles are exported as `d8_user_authz_foreign_aggregation_label{name, lineage, kind}` and raise the
`D8UserAuthzForeignAggregationLabel` alert; list them before upgrading:

```bash
d8 k get clusterroles -o json | jq -r '.items[]
  | select((.metadata.labels // {}) | to_entries | any(.key | startswith("rbac.deckhouse.io/aggregate-to-")))
  | select((.metadata.labels.heritage // "") != "deckhouse")
  | select((.metadata.labels["rbac.deckhouse.io/kind"] // "") != "custom-capability")
  | .metadata.name'
```

For each one either make it a proper custom capability (`d8:custom:<scope>-capability:<name>`,
`rbac.deckhouse.io/kind: custom-capability`, `rbac.deckhouse.io/scope: <scope>`, no `aggregationRule`)
or drop the `aggregate-to-*-as` labels.

## What happens if a module is not migrated

Nothing rejects the old objects and nothing is logged — the labels are simply read by different code
now. Three consequences:

- **A `use` capability is granted cluster-wide instead of per namespace, or not at all.** Its lineage
  label now belongs to a cluster-scoped subsystem or to none. A label of a subsystem the role model
  kept (`observability`, `security`, `storage`) lands the module's namespaced rules in
  `d8:subsystem:<subsystem>:*` and, up the ladder, in `d8:system:*`, so every holder of those roles
  gets them across the whole cluster. A label of `deckhouse`, `infrastructure`, `kubernetes` or
  `networking` is selected by no role of the model. Either way `d8:namespace:*` no longer contains the
  rules, so tenants lose the access they had.
- **A `use` capability at the `user` or `admin` level disappears.** The subsystem lineages have no such
  levels, nothing selects the object, and its rules go nowhere.
- **The automatic `RoleBinding` in the module's namespace stops being created.** The hook that fans a
  `ClusterRoleBinding` on a system or subsystem role out into namespaced `RoleBinding` objects only
  looks at objects labelled `rbac.deckhouse.io/scope: system` or `subsystem`, so without that label the
  `rbac.deckhouse.io/namespace` label is never read.

A `manage` capability on `observability`, `security` or `storage` keeps aggregating into the same
subsystem role, so its permissions do not move — but it stays invisible to the console and to the
permission browser, which classify objects by `rbac.deckhouse.io/kind` and `rbac.deckhouse.io/scope`.
On `deckhouse`, `infrastructure`, `kubernetes` or `networking` it reaches no role at all, and its
permissions are gone until the module moves it to the [subsystem of the role model](#the-subsystems).
