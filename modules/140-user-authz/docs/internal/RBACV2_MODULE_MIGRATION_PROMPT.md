# Agent prompt: migrate a module to the RBACv2 role model of DKP 1.78

Paste everything below the line into a coding agent opened in the module repository. It is
self-contained: the agent needs no other document, and the module name and paths are discovered by
the agent itself.

The agent does the same rewrite as `rbacv2-migrate-module.sh`, plus the three decisions the script
deliberately leaves to a human — which tier a rule belongs to, which level a capability aggregates
at, and what to do with a second aggregation lineage. If you only want the mechanical part, run the
script and read [RBACV2_MODULE_MIGRATION.md](./RBACV2_MODULE_MIGRATION.md) instead.

Teams working in Cursor can keep it as a skill instead: save the text below the line as
`.cursor/skills/rbacv2-migration/SKILL.md`, prefixed with a frontmatter block carrying `name:
rbacv2-migration` and a one-line `description`, and invoke it by name.

---

## Task

Migrate this module's RBACv2 templates from the legacy `manage`/`use` scheme to the role model of
DKP 1.78. The rewrite is mostly mechanical — object names, labels and annotations — but three
decisions need judgement, and getting them wrong changes who has access to what.

**Mistakes here are silent.** Nothing rejects these objects at apply time and nothing is logged: the
labels are simply read by different code now. Three things happen to a module that gets this wrong,
all of them without a single error message:

- a namespaced capability that keeps `aggregate-to-kubernetes-as` lands in the cluster-wide
  `d8:subsystem:kubernetes:*` roles, so its rules are granted across the whole cluster while the
  tenants who used to have them in their namespace lose them;
- a namespaced capability at the `user` or `admin` level disappears entirely, because the subsystem
  lineages have no such levels and nothing selects the object;
- without `rbac.deckhouse.io/scope`, the automatic `RoleBinding` in the module's own namespace stops
  being created, because the hook that fans out cluster bindings only looks at scoped objects.

So verify with the commands in "Acceptance"; a diff that looks plausible proves nothing here.

## Scope

Change only:

- files under `**/templates/rbacv2/**`;
- inside those files, only `metadata` (the object name, the `rbac.deckhouse.io/*` labels, the i18n
  annotations) — and `rules`, only when you move a rule between tiers as decided below.

Do not:

- touch any cluster, run `kubectl`/`d8 k` against one, or apply anything;
- rename or move files, reorder keys, reindent, or reformat anything you are not changing — the diff
  must stay reviewable line by line;
- edit module documentation, CI, chart values or Go code;
- invent labels or annotations beyond the ones listed here.

## Step 1. Inventory

Run these first and base everything on what they return:

```shell
# The files in question.
find . -type f -name '*.yaml' -path '*/templates/rbacv2/*' | sort

# Are they already migrated? (legacy markers)
grep -rn 'rbac.deckhouse.io/kind: \(use\|manage\)\|d8:use:capability\|d8:manage:permission' \
  --include='*.yaml' .

# Are the labels templated through helm_lib_module_labels?
grep -rln 'helm_lib_module_labels' --include='*.yaml' . | grep 'templates/rbacv2'

# The module's own CRDs, needed for the tier decision below.
grep -rn 'scope: \(Cluster\|Namespaced\)' crds/ 2>/dev/null
```

If the first command returns nothing, the module ships no RBACv2 templates: report that and stop.

The legacy layout is `templates/rbacv2/{manage,use}/{view,edit}.yaml`, one `ClusterRole` per file.
Anything that does not match — several objects in one file, an action other than `view`/`edit`, a
file that defines a role (it has `aggregationRule` and no `rules`) — is out of the script's reach
and probably out of the ordinary: handle it by the rules below if you can, and report it either way.

## Step 2. The model you are migrating to

Permissions live in **capabilities**: `ClusterRole` objects that carry `rules` plus one or more
`rbac.deckhouse.io/aggregate-to-<lineage>-as: <level>` labels. **Roles** are empty `ClusterRole`
objects whose `aggregationRule` selects those labels; Kubernetes fills them in. A module ships
capabilities only — the roles belong to `user-authz` and `multitenancy-manager`.

If you do end up touching a role, its manifest must carry no `rules` key at all — not even
`rules: []`. That field belongs to the `clusterrole-aggregation-controller`, and a chart that
declares it fights the controller for it: the apply fails with
`conflict with "clusterrole-aggregation-controller": .rules`, and forcing the conflict rewrites the
object twice per reconcile, leaving the role empty in between.

| Lineage | Levels | Granted with | What a module puts there |
|---------|--------|--------------|--------------------------|
| `namespace` | `viewer`, `user`, `manager`, `admin`, `superadmin` | `RoleBinding` | The module's namespaced resources — what a tenant works with |
| a subsystem: `deckhouse`, `infrastructure`, `kubernetes`, `networking`, `observability`, `security`, `storage` | `viewer`, `manager`, `superadmin` | `ClusterRoleBinding` | The module's cluster-scoped resources and its `ModuleConfig` |

The legacy scheme had the same two tiers under different names: `use` for what happens inside a
namespace, `manage` for the module's own configuration. What changes is that the namespace tier
moves off the `kubernetes` subsystem onto the `namespace` lineage, where a `RoleBinding` can reach it.

## Step 3. The mechanical rewrite

Per file, rename the object and fix the labels:

| Legacy | New |
|--------|-----|
| `d8:use:capability:module:<module>:<action>` | `d8:namespace-capability:<module>:<action>` |
| `d8:manage:permission:module:<module>:<action>` | `d8:system-capability:<module>:<action>` |
| `rbac.deckhouse.io/kind: use` or `manage` | `rbac.deckhouse.io/kind: capability` |
| — | `rbac.deckhouse.io/scope: namespace` (was `use`) or `system` (was `manage`) |
| — | `rbac.deckhouse.io/capability: "<scope>-capability.<module>.<action>"` |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as: <level>` on a `use` object | `rbac.deckhouse.io/aggregate-to-namespace-as: <level>`, same level |
| `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` on a `manage` object | unchanged, including several at once |
| `rbac.deckhouse.io/level: module` | delete it, nothing reads it |
| `rbac.deckhouse.io/namespace: d8-<module>` | keep on `system`, delete on `namespace` (it is only read on `system`/`subsystem` objects) |

The `rbac.deckhouse.io/capability` marker is what lets a custom role or the console select exactly
this capability, so it must be unique across the whole platform — keep the module name inside it. It
is a Kubernetes label value: at most 63 characters, `[A-Za-z0-9]` at both ends, dots, dashes and
underscores in between.

Then add the four localized annotations, using this wording verbatim (`<module>` is the module name):

| Tier and action | `en.…/title` | `ru.…/title` | `en.…/description` | `ru.…/description` |
|---|---|---|---|---|
| use/view | `Module <module>: view` | `Модуль <module>: просмотр` | `Read-only access to <module> resources in a namespace.` | `Доступ только на чтение к ресурсам модуля <module> в пространстве имён.` |
| use/edit | `Module <module>: edit` | `Модуль <module>: редактирование` | `Manage <module> resources in a namespace.` | `Управление ресурсами модуля <module> в пространстве имён.` |
| manage/view | `Module <module>: view configuration` | `Модуль <module>: просмотр конфигурации` | `Read-only access to the <module> module configuration.` | `Доступ только на чтение к конфигурации модуля <module>.` |
| manage/edit | `Module <module>: edit configuration` | `Модуль <module>: управление конфигурацией` | `Manage the <module> module configuration.` | `Управление конфигурацией модуля <module>.` |

If the file already has `meta.deckhouse.io/title` annotations, keep them and only report the wording.

### Example: plain YAML

Before:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/aggregate-to-kubernetes-as: viewer
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/level: module
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

### Example: labels templated through helm_lib_module_labels

The same pairs change, as `"key" "value"` arguments inside the `dict` call; the annotations are
added as plain YAML next to the name:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  {{- include "helm_lib_module_labels" (list . (dict "rbac.deckhouse.io/capability" "system-capability.mymodule.edit" "rbac.deckhouse.io/aggregate-to-networking-as" "manager" "rbac.deckhouse.io/kind" "capability" "rbac.deckhouse.io/scope" "system" "rbac.deckhouse.io/namespace" "d8-mymodule")) | nindent 2 }}
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

## Step 4. The three decisions

Make each one explicitly, and report every one you make with the reason.

1. **Does every rule sit in the right tier?** A `namespace` capability aggregates into namespace
   roles, which are granted with a `RoleBinding` — a rule on a cluster-scoped resource there grants
   nothing at all. The legacy scheme hid this, because a `use` capability used to aggregate into a
   cluster-wide subsystem role where such a rule did work. So read every rule of every former `use`
   file and ask whether the resource is namespaced.

   Decide it from evidence, in this order: the module's own CRDs (`spec.scope` in `crds/`), then
   Kubernetes' own scoping (`nodes`, `namespaces`, `persistentvolumes`, `storageclasses`,
   `clusterroles`, `clusterrolebindings`, `customresourcedefinitions`, `priorityclasses`,
   `ingressclasses`, `apiservices`, `csidrivers` and the webhook configurations are cluster-scoped),
   then the platform's `deckhouse.io` resources (`moduleconfigs`, `modules`, `nodegroups`, the
   `*instanceclasses` are cluster-scoped). A cluster-scoped rule moves into the matching file under
   `manage/`; create that file only if the module has none, and say so. If moving the rules empties
   the `use` file, delete it — a capability without `rules` fails the contract — and report that the
   module has no namespace tier left. If you cannot establish the scope of a resource, leave the rule
   where it is and report the question — do not guess.

2. **Is the level still right?** The script keeps whatever it finds, and almost every module in the
   platform ended up with `view` → `viewer` and `edit` → `manager`. Deviate only with a reason:

   - `viewer` — read-only.
   - `user` — a tenant working in the namespace day to day. If your `edit` capability grants the
     resources such a tenant creates and updates by themselves, `user` is the level for it.
   - `manager` — namespace-wide settings: quotas, limits, policies. The default for `edit`.
   - `admin` — RBAC inside the namespace. A module capability almost never belongs here.
   - `superadmin` — system objects a tenant must not reach. Only if the capability really grants
     access to the platform's own workload.

3. **Was a second lineage dropped?** A legacy `use` capability sometimes carried two labels at once
   (`kubernetes` and `networking`, say). The namespace tier takes exactly one
   `aggregate-to-namespace-as`, so the second lineage disappears. If the module genuinely wants its
   cluster-scoped part in that subsystem, it belongs in a separate `system` capability — report this
   rather than silently losing it. If the two labels disagreed on the level, say which one you kept.

## Step 5. Acceptance

The migration is done when all of these pass. Run them; do not report success from reading the diff.

```shell
# 1. Nothing legacy is left.
grep -rn 'rbac.deckhouse.io/kind: \(use\|manage\)\|d8:use:capability\|d8:manage:permission\|rbac.deckhouse.io/level' templates/

# 2. The chart still renders.
helm template . --set global.enabledModules='{}' >/dev/null

# 3. The diff is confined to the templates you were asked to touch.
git diff --name-only
```

If the chart does not render for a reason unrelated to your change — a missing dependency, values the
module expects — say so in the report and leave it alone; do not edit values or templates to make the
command pass.

And every migrated object satisfies the contract the platform tests
(`go test -tags validation ./testing/rbacv2/` in the `deckhouse` repository):

- the name starts with `d8:` and with the prefix of its scope — `d8:namespace-capability:` or
  `d8:system-capability:`;
- `rbac.deckhouse.io/kind: capability` and a `rbac.deckhouse.io/scope` of `system` or `namespace`;
- all four i18n annotations are present;
- it has `rules` and no `aggregationRule` (that is what distinguishes a capability from a role; a
  role is the other way round and carries no `rules` key at all);
- it carries a unique `rbac.deckhouse.io/capability` marker, at most 63 characters;
- it carries at least one `rbac.deckhouse.io/aggregate-to-<lineage>-as` label, whose lineage is
  `namespace`, `system`, `project` or a subsystem, and whose level is one of `viewer`, `user`,
  `manager`, `admin`, `superadmin`;
- `rbac.deckhouse.io/delegatable` appears on no capability at all.

## Step 6. Report

Finish with a short report, in this shape:

```text
Migrated: <n> files
  <path> — <old name> → <new name>, aggregates to <lineage> as <level>

Rules moved between tiers: <n>
  <path> — <apiGroup>/<resource> is cluster-scoped, moved to <path>, because <evidence>

Levels changed: <n>
  <path> — <old> → <new>, because <reason>

Dropped lineages: <n>
  <path> — <lineage> dropped; <needs a system capability | not needed, because ...>

Open questions: <n>
  <path> — <what you could not decide and what you need to know>

Checks: legacy markers <none|list>, helm template <ok|error>, files outside scope <none|list>
```

Stop and ask instead of guessing when: a file defines a role rather than a capability; an object does
not match the legacy naming pattern; the module ships actions other than `view`/`edit`; a capability
has no aggregation label at all (it would land in no role); or the scope of a resource cannot be
established from the repository.
