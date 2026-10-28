# Agent prompt: migrate a module to the RBACv2 role model of DKP 1.78

Paste everything below the line into a coding agent opened in the module repository. It is
self-contained: the agent needs no other document, and the module name and paths are discovered by
the agent itself.

The agent does the same rewrite as `rbacv2-migrate-module.sh`, plus the three decisions the script
deliberately leaves to a human — which tier a rule belongs to, which level a capability aggregates
at, and what to do with a second aggregation lineage. If you only want the mechanical part, run the
script and read [RBACV2_MODULE_MIGRATION.md](./RBACV2_MODULE_MIGRATION.md) instead.

Like the script, it keeps the legacy object and puts the new one beside it behind a version gate, so
one branch of the module serves clusters on either side of 1.78. A module that supports only 1.78
and later does not need that; tell the agent so, and it drops the legacy branch.

The agent checks the result with the `contract` rule of dmt v0.2.7 or newer when dmt is installed.
Moving the module's RBAC to `rbac.yaml` is a separate step (Step 7): the agent does it only when you
ask for it, after the migration is committed.

Teams working in Cursor can keep it as a skill instead: save the text below the line as
`.cursor/skills/rbacv2-migration/SKILL.md`, prefixed with a frontmatter block carrying `name:
rbacv2-migration` and a one-line `description`, and invoke it by name.

---

## Task

Migrate this module's RBACv2 templates from the legacy `manage`/`use` scheme to the role model of
DKP 1.78, **keeping the legacy objects for clusters below 1.78**. The rewrite is mostly mechanical —
object names, labels and annotations — but three decisions need judgement, and getting them wrong
changes who has access to what.

This module is installed on clusters of more than one platform version, and the two role models are
not compatible: an object of one model is invisible to the other. So each template ends up carrying
both objects behind a gate on `global.deckhouseVersion` — the new one for DKP 1.78 and later, the
legacy one below it. Drop the legacy branch only if the module's `module.yaml` already requires
1.78 or newer, or the person running you says so.

**Mistakes here are silent.** Nothing rejects these objects at apply time and nothing is logged: the
labels are simply read by different code now. Four things happen to a module that gets this wrong,
all of them without a single error message:

- a namespaced capability that keeps the label of a subsystem the role model kept (`observability`,
  `security`, `storage`) lands in the cluster-wide `d8:subsystem:<subsystem>:*` roles, so its rules are
  granted across the whole cluster, and one that keeps the label of a legacy subsystem
  (`aggregate-to-kubernetes-as`) lands in no role; either way the tenants who used to have the rules in
  their namespace lose them;
- a cluster-level capability that keeps the label of a legacy subsystem (`deckhouse`,
  `infrastructure`, `kubernetes`, `networking`) lands in no role at all;
- a namespaced capability at the `user` or `admin` level disappears entirely, because the subsystem
  lineages have no such levels and nothing selects the object;
- without `rbac.deckhouse.io/scope`, the automatic `RoleBinding` in the module's own namespace stops
  being created, because the hook that fans out cluster bindings only looks at scoped objects.

So verify with the commands in "Acceptance"; a diff that looks plausible proves nothing here.

## Scope

Change only:

- files under `**/templates/rbacv2/**`;
- inside those files, only `metadata` (the object name, the `rbac.deckhouse.io/*` labels, the i18n
  annotations) — and `rules`, only when you move a rule between tiers as decided below;
- one new file, `templates/_rbacv2_compat.tpl`, holding the version gate;
- files under `templates/rbacv2-compat/`, only for the aliases the person running you asks for (Step 3c).

Do not:

- touch any cluster, run `kubectl`/`d8 k` against one, or apply anything;
- rename or move files, reorder keys, reindent, or reformat anything you are not changing — the diff
  must stay reviewable line by line;
- edit module documentation, CI, chart values or Go code;
- invent labels or annotations beyond the ones listed here;
- run `dmt lint --fix` for Steps 1–6. From dmt v0.2.7 on, `--fix` writes `rbac.yaml` into the module
  and then rewrites its RBAC templates from it, which is Step 7 and only on request. In Steps 1–6 run
  `dmt lint` without `--fix`.

Step 7 widens this scope, and only when the person running you asks for it.

## Step 1. Inventory

Run these first and base everything on what they return:

```shell
# The files in question.
find . -type f -name '*.yaml' -path '*/templates/rbacv2/*' | sort

# Are they already migrated? (legacy markers)
grep -rn 'rbac.deckhouse.io/kind: \(use\|manage\)\|d8:use:capability\|d8:manage:permission' \
  --include='*.yaml' .

# Is the version gate already there? (a previous run, or a hand-written one)
grep -rn 'rbacv2_new_scheme' --include='*.yaml' --include='*.tpl' .

# Does the module already require 1.78 or newer? Then the legacy branch is dead weight.
grep -n 'deckhouse:' module.yaml 2>/dev/null

# Which subsystem does module.yaml declare? It must be the module's subsystem in the DKP subsystems table.
grep -n -A3 '^subsystems' module.yaml 2>/dev/null

# Are the labels templated through helm_lib_module_labels?
grep -rln 'helm_lib_module_labels' --include='*.yaml' . | grep 'templates/rbacv2'

# The module's own CRDs, needed for the tier decision below.
grep -rn 'scope: \(Cluster\|Namespaced\)' crds/ 2>/dev/null
```

If the first command returns nothing, the module ships no RBACv2 templates: report that and stop.
If the gate is already there, the file was migrated before: leave it alone and say so.

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
| a subsystem, one of the eight the platform ships (see below), or one of the module's own that its `module.yaml` declares and whose `d8:subsystem:<name>:*` roles the module ships (see RBACV2_MODULE_MIGRATION.md, "A subsystem of the module's own") | `viewer`, `manager`, `superadmin` | `ClusterRoleBinding` | The module's cluster-scoped resources and its `ModuleConfig` |

The legacy scheme had the same two tiers under different names: `use` for what happens inside a
namespace, `manage` for the module's own configuration. What changes is that the namespace tier
moves off the `kubernetes` subsystem onto the `namespace` lineage, where a `RoleBinding` can reach it.

### The subsystems

The role model has eight subsystems. They are `iam`, `security`, `cluster`, `delivery`, `network`,
`storage`, `observability` and `managed-services`. The `d8:subsystem:<subsystem>:*` roles of all of them
come from user-authz, and the module never ships one of those. The DKP subsystems table, an internal
document, puts every DKP module in one of them, and `subsystems` in `module.yaml` must name that one.
If it does not, or names a legacy subsystem, report it and ask; do not guess the module's subsystem.

The seven legacy subsystems went to the following ones.

| Legacy subsystem | Subsystem of the role model |
|------------------|-----------------------------|
| `deckhouse`, `infrastructure`, `kubernetes` | `cluster` |
| `networking` | `network` |
| `observability`, `security`, `storage` | the same |
| a subsystem of the module's own (`virtualization`) | the one `module.yaml` declares (`cluster` for virtualization) |

No role of the model aggregates a legacy lineage, and a capability never carries the legacy label and
the new one together. A module that had a subsystem of its own named its objects after the module
(`d8:manage:permission:subsystem:<module>:<action>`, `aggregate-to-<module>-as`). If `module.yaml`
now declares a platform subsystem instead, the module's system capabilities move to that subsystem's
lineage, and its roles `d8:manage:<module>:<level>` have no counterpart (Step 3c).

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
| `rbac.deckhouse.io/aggregate-to-<subsystem>-as: <level>` on a `manage` object | when `module.yaml` declares one subsystem of the role model, the label of that subsystem, same level, whichever subsystem the old label named; otherwise the label of the subsystem of the role model that took the legacy one over (Step 2), same level, and report a label of a subsystem `module.yaml` does not declare. Two labels that land on one subsystem are merged into one and reported. An `rbac.yaml` that lists `subsystems` takes the place of `module.yaml` here |
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
  {{- include "helm_lib_module_labels" (list . (dict "rbac.deckhouse.io/capability" "system-capability.mymodule.edit" "rbac.deckhouse.io/aggregate-to-network-as" "manager" "rbac.deckhouse.io/kind" "capability" "rbac.deckhouse.io/scope" "system" "rbac.deckhouse.io/namespace" "d8-mymodule")) | nindent 2 }}
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

## Step 3b. Keep both schemes behind the version gate

Write `templates/_rbacv2_compat.tpl` once, with the module name in the define (`<module>` is the
chart name, the same one that appears in the object names):

```text
{{- define "<module>.rbacv2_new_scheme" -}}
  {{- $raw := (.Values.global).deckhouseVersion | default "dev" | toString -}}
  {{- $mm := regexFind "^v?[0-9]+[.][0-9]+" $raw -}}
  {{- if $mm -}}
    {{- semverCompare ">= 1.78" (printf "%s.0" $mm) -}}
  {{- else -}}
    {{- /* "dev" or "unknown": a build off any branch says the same, so answer with the model
           whose mistake only loses access. A dev stand below 1.78 flips this to false. */ -}}
    true
  {{- end -}}
{{- end -}}
```

Copy it as it is. Six things in it are not decoration:

- `global.deckhouseVersion` is `dev` on a development build and `unknown` when the version file is
  missing. `semverCompare` raises on both, and a raise takes down the render of the whole module, not
  just this file, so anything that is not a version has to answer without comparing.
- It answers `true`, and that is about the direction of failure rather than about which branch is
  likelier. The version file holds `CI_COMMIT_TAG`, so a build off any branch says `dev`: a dev stand
  built from `release-1.77` looks exactly like one built from `main`. Of the two ways to be wrong
  there, only one is dangerous. The new object on a cluster of the legacy scheme is inert, because
  nothing there selects `aggregate-to-namespace-as`, and the module's permissions are merely missing; a
  legacy use capability on a cluster of the role model that carries the lineage of a subsystem the role model kept
  (`observability`, `security`, `storage`) is aggregated by `d8:subsystem:<subsystem>:<level>`, which
  is granted through a `ClusterRoleBinding`, so rules meant for one namespace become cluster-wide. One
  direction loses access, the other hands it out.
- The comparison is on major and minor only. Semver orders `1.78.0-rc.1` **below** `1.78.0`, so
  comparing the raw string would serve a release candidate of 1.78 the legacy object.
- The regex uses `[.]` rather than `\.`: a backslash escape inside a Go template string literal is a
  parse error.
- `.Values.global` is in parentheses. Without them a chart rendered with no global values at all —
  which is how a module is often checked locally — fails on a nil pointer instead of falling back.
- `toString` is there because `deckhouseVersion: 1.78` unquoted in a values file is a YAML float, and
  `regexFind` on a non-string kills the render. `--set` is safe, a values file is not, and pinning the
  version in a values file is exactly what a template test does.

Then wrap each migrated file, new object first:

```text
{{- if eq (include "<module>.rbacv2_new_scheme" .) "true" }}
<the migrated object>
{{- else }}
<the original object, unchanged>
{{- end }}
```

The legacy object keeps its old name, labels and rules verbatim — it is what the cluster below 1.78
reads. Do not "fix" it, and do not leave a file where both branches carry the same object.

A file that already carries the gate was migrated before. If its new branch still names a legacy
subsystem (`aggregate-to-infrastructure-as`), move that label as in Step 3 in the new branch only, and
leave the legacy branch as it is. If its new branch holds `d8:subsystem:<module>:<level>` and
`module.yaml` now puts the module in a platform subsystem, empty the new branch, keep the gate and the
legacy branch, and treat the legacy name `d8:manage:<module>:<level>` as Step 3c says.

## Step 3c. Roles of the module

A role of the module is an `aggregationRule` and no rules. A namespace role of the module
(`d8:use:role:<level>:<module>`, `d8:use:<module>:superadmin`) has no counterpart in the role model,
since `d8:namespace:<level>` gathers the namespace capabilities. A subsystem role of the module
(`d8:manage:<module>:<level>`) keeps one while `module.yaml` declares the module's own subsystem, and
then it becomes `d8:subsystem:<module>:<level>` (see RBACV2_MODULE_MIGRATION.md, "A subsystem of the
module's own"). Once `module.yaml` puts the module in a platform subsystem, the role has no counterpart
either, since `d8:subsystem:<subsystem>:<level>` of user-authz gathers the system capabilities. A role
with no counterpart stays in the legacy branch of the gate only. Do not decide on your own whether its
old name stays; report the role and ask. If the person running you wants the name kept, write it as an
alias in `templates/rbacv2-compat/`, one ClusterRole per old name:

- an empty `ClusterRole` with the old name, no `rules` key, the label `rbac.deckhouse.io/deprecated: "true"`,
  the annotation `rbac.deckhouse.io/deprecated-replaced-by: <new role>`, and the four i18n annotations
  (`Deprecated: <old name>`);
- selectors that grant no more than the role did, by `module: <module>`, `rbac.deckhouse.io/kind: capability`
  and the lineage label, one selector per level up to the role's own;
- for a namespace role (`d8:use:role:<level>:<module>`, `d8:use:<module>:superadmin`, replaced by
  `d8:namespace:<level>`), the selector label is `rbac.deckhouse.io/aggregate-to-namespace-as`, and the
  alias also carries the label `rbac.deckhouse.io/delegatable: "true"` and the annotation
  `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"`, so a RoleBinding to it inside a
  project namespace keeps working and a new ProjectRoleBinding to it is refused;
- for a subsystem role (`d8:manage:<module>:<level>`, replaced by `d8:subsystem:<subsystem>:<level>`),
  the selector adds `rbac.deckhouse.io/scope: system` and uses
  `rbac.deckhouse.io/aggregate-to-<subsystem>-as`, and the alias carries neither
  `rbac.deckhouse.io/delegatable` nor `rbac.deckhouse.io/disabled-for-direct-use-in-projects`.

`templates/rbacv2-compat/` is outside the version gate, while the legacy branch still renders the role
under the old name below the version the gate checks. Wrap each alias in the gate
(`{{- if eq (include "<module>.rbacv2_new_scheme" .) "true" }}` ... `{{- end }}`), or the two objects
collide on those clusters. With the legacy branch dropped the alias needs no gate.

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
# 1. Every legacy object sits in the else branch of a gate, and none outside it.
grep -rn 'd8:use:capability\|d8:manage:permission' templates/
grep -rn 'rbacv2_new_scheme' templates/

# 2. Both branches render, and each gives the objects of its own model.
helm template . --set global.deckhouseVersion=1.77.5 --set global.enabledModules='{}' | grep 'name: d8:'
helm template . --set global.deckhouseVersion=v1.78.0 --set global.enabledModules='{}' | grep 'name: d8:'
helm template . --set global.deckhouseVersion=dev     --set global.enabledModules='{}' | grep 'name: d8:'

# 3. The diff is confined to the templates you were asked to touch, plus the gate.
git diff --name-only
```

The first render must list only `d8:use:capability:*` and `d8:manage:permission:*`, the second and
the third only `d8:namespace-capability:*` and `d8:system-capability:*`. A render that fails on
`dev` means the gate was not copied as written. The second render must also carry no legacy subsystem
label:

```shell
helm template . --set global.deckhouseVersion=v1.78.0 --set global.enabledModules='{}' \
  | grep 'aggregate-to-\(deckhouse\|infrastructure\|kubernetes\|networking\)-as'
```

The module's own template tests are the one thing outside `templates/` the gate does break, and they
break silently in the sense that matters: a test that looks up a ClusterRole by name now finds it
only in the branch its render selects. A harness that leaves `global.deckhouseVersion` unset renders
the new scheme, so every assertion written against a legacy name fails. Report which tests reference
these names; do not rewrite them unless you are asked to. The fix is for the test to pin the version
it means:

```shell
grep -rn 'd8:use:capability\|d8:manage:permission\|namespace-capability\|system-capability' \
  --include='*_test.go' --include='*_test.py' --include='*.yaml' . | grep -v templates/
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
  `namespace`, `system`, `project` or one of the eight subsystems, and whose level is one of `viewer`,
  `user`, `manager`, `admin`, `superadmin`;
- no legacy subsystem label is left on it (`deckhouse`, `infrastructure`, `kubernetes`, `networking`);
- `rbac.deckhouse.io/delegatable` appears on no capability at all.

If dmt v0.2.7 or newer is installed, check the same contract with it. Its `contract` rule is a port
of `testing/rbacv2` and needs no `rbac.yaml`:

```shell
dmt --version
dmt lint --linter rbac .          # without --fix
```

There must be no `contract` finding. A legacy object behind the version gate is not reported, so a
module that serves both models passes. Check first that the module's `.dmtlint.yaml` does not set
`linters-settings.rbac.impact` below `warn` (`ignored`): `contract` falls back to it and does not run,
and no findings then proves nothing; report such a setting instead of relying on the check. A finding about `module.yaml` `subsystems` means they differ
from the subsystems the system capabilities aggregate into: fix the label or report it, do not edit
`module.yaml` on your own. The `sync` finding "rbac.yaml is missing" is expected and is not part of
this migration (Step 7). If dmt is older or not installed, say so in the report; do not install it.

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

Roles of the module: <n>
  <path> — <old name>, no counterpart; alias <asked for and written | not wanted>

Gate: <written|dropped, because the module requires 1.78+>

Tests referencing role names: <n>
  <path:line> — <the name it expects>

Checks: renders at 1.77.5 / 1.78.0 / dev <objects of the expected model | error>,
        files outside scope <none|list>,
        dmt contract <no findings | n findings, listed above | dmt <version> too old or not installed>
```

Stop and ask instead of guessing when: a file defines a role rather than a capability; an object does
not match the legacy naming pattern; the module ships actions other than `view`/`edit`; a capability
has no aggregation label at all (it would land in no role); or the scope of a resource cannot be
established from the repository.

## Step 7. Only on request: the module's RBAC in rbac.yaml

Do this step only when the person running you asks for it, and only after the migration above is
committed. It moves the module's whole RBAC into one declaration, `rbac.yaml` in the module root, from
which dmt writes `templates/rbacv2/`, `templates/user-authz-cluster-roles.yaml`,
`templates/**/rbac-for-us.yaml` and `templates/rbac-to-us.yaml`. The format is in the README of the
`rbac` linter of dmt (github.com/deckhouse/dmt, `pkg/linters/rbac/README.md`, section "The module RBAC
declaration"); read it before you start.

The result is two commits, and the line between them matters: commit 1 is only what dmt wrote, so
anyone can reproduce it; commit 2 is what a person decided. Never edit a file by hand before commit 1,
and never put a decision of yours into it.

### 7.1. Preconditions

Check all of these and stop if one fails:

```shell
dmt --version                       # v0.2.7 or newer
dmt lint --linter rbac .            # no contract findings (Step 5)
git status --short                  # empty: commit 1 must hold only dmt output
```

Find out which values render every RBAC object. Look for `{{ if }}` around RBAC objects in
`templates/` and in the openapi of the module for the flags they test. If some objects are off with the
default values, prepare a values file that turns them on and use it as `--values-file` in every run, or
use `--matrix`. Name the values you chose in the report. Do not run `dmt lint --fix` without
`--linter rbac`: it would apply the fixes of other linters too.

If the module keeps both schemes behind the version gate (Step 3b, `rbacv2_new_scheme` in
`templates/`), dmt takes this step only part of the way, by design and not as a bug:

- `sync` compares the 1.78 branch of a gated template with `rbac.yaml` and skips the legacy branch;
- `--fix` never rewrites a gated template, since the rewrite would drop the legacy branch;
- run 1 does not take the rights of the gated `templates/rbacv2/` into `rbac.yaml`, so their resources
  get `noAccess: "TODO"`.

Say so to the person running you before you start. For such a module, the levels of those resources
are written into `rbac.yaml` by hand after the 1.78 branch (a decision of 7.3), the gated templates stay
hand-written, and their `sync` finding "The autofix leaves the file as it is" goes away once the
declaration and the 1.78 branch agree. Do not edit a gated template to make a finding go away.

### 7.2. Commit 1: the bare dmt run

The move takes two fixes that depend on each other: write `rbac.yaml` from the templates, then the
templates from `rbac.yaml`. A run of `--fix` applies only the fixes of what `dmt lint` found at the start
of that run and does not lint again, so the runs go in a chain:

- run 1 writes `rbac.yaml` from the render and touches no template (there was no declaration at the
  start of the run);
- run 2 compares the templates with the new declaration, rewrites those that differ and creates the
  missing files; a template it cannot rewrite without losses is left as it is, with a finding;
- the first run that changes nothing is the last one.

Run this loop and nothing else:

```shell
git add -A .
for i in 1 2 3 4 5; do
  dmt lint --linter rbac --fix . >/dev/null      # plus --values-file <file> or --matrix, the same every run
  if [ -z "$(git status --porcelain . | grep -v '^[MADR] ')" ]; then
    echo "converged after $i run(s)"; break
  fi
  git add -A .
  [ "$i" = 5 ] && echo "NOT converged after 5 runs: stop and report"
done
[ -f rbac.yaml ] || echo "no rbac.yaml: dmt did not run, see its errors above"
git status --short .
```

The loop leaves the errors of dmt on the terminal. If dmt printed an error (no dmt, a render error, a
wrong values file), the loop proves nothing even when it says "converged": stop and report the error.
If it prints "NOT converged after 5 runs", stop and report it as a dmt bug, with the files each run
changed. Otherwise commit the staged files as commit 1, with a message that says it is the bare output
of `dmt lint --linter rbac --fix`, how many runs it took and with which values.

Then read what dmt wrote and what it reports:

```shell
sed -n '1,/^apiVersion:/p' rbac.yaml        # the notes on top: what the declaration does not carry
grep -n 'TODO' rbac.yaml
dmt lint --linter rbac .
```

### 7.3. Commit 2: the decisions

Sort every `TODO`, note and finding into one of two groups.

Do it yourself, mechanically, and list it in the report:

- a template the declaration puts into another file (dmt never moves objects between files and never
  deletes a file): move or rename the file as the finding says, so that the object is not described
  twice;
- the comments of the templates: carry them next to the matching entries of `rbac.yaml`, and add a
  header to `rbac.yaml` naming the templates dmt writes from it;
- a tool of the repository that reads the moved or renamed templates (a generator, a test, an audit
  policy): rerun it and include its output, or report it if you cannot.

Ask the person running you, with your proposal and its evidence, and do not decide alone:

- every `noAccess: "TODO"`: grant levels to the CRD, or which reason to write. Propose "no user role
  grants it today" only after you have checked the templates for such grants;
- every other `TODO` (`scope` of a resource without a CRD, a `when` dmt could not read, a namespaced
  resource at the system level): say what the templates do today. Keep what they do today unless told
  otherwise: this step must not change who has access;
- anything the format cannot carry that keeps `--fix` off a file (`imagePullSecrets` of a
  ServiceAccount, a Role that a declared account only binds but that lives in its `rbac-for-us.yaml`,
  labels or annotations of a legacy role or capability): propose the change, for example moving
  `imagePullSecrets` into the pod spec or moving the Role to a file the declaration does not write;
- an object dmt writes from the declaration that the templates did not have before, such as the
  system capabilities `view`/`edit` of a subsystem: say what access it would add. Excluding it from
  `sync` in `.dmtlint.yaml` (`linters-settings.rbac.exclude-rules.sync`) keeps `--fix` from writing it
  back.

After the answers, edit `rbac.yaml` and the files above, then run the loop from 7.2 again with the same
values: an edit of the declaration is rewritten into the templates by the next run, and what follows
from that rewrite is seen only by the run after it. Commit the result as commit 2, with a message that
lists each decision and who took it.

### 7.4. Acceptance

```shell
dmt lint --linter rbac .                          # nothing: no TODO, no sync, no coverage finding
dmt lint --linter rbac --fix . && git status -s   # changes nothing
```

Run both with the default values and with the values of the loop (and `--matrix` if you used it). For
a module behind the version gate (7.1), the gated templates must agree with `rbac.yaml` too: no `sync`
finding on them either, and the legacy branch must render as before.
Then render the chart on the base commit and on the result with the same values, and compare the RBAC
objects (kind, name, namespace, rules, roleRef, subjects). They must be the same except for what
commit 2 changes on purpose; list each difference in the report.

### 7.5. Report for Step 7

```text
rbac.yaml: commit 1 <sha>, <n> dmt runs, values <default | file | --matrix>
  files dmt wrote: <list>
Decisions (commit 2 <sha>): <n>
  <rbac.yaml path or entry> — <decision>, <taken by: person | mechanical>
Left hand-written: <objects the declaration does not own, with the reason dmt gave>
Checks: rbac lint <clean | findings>, --fix <no change | changes>,
        render vs base <same | differences listed>
```
