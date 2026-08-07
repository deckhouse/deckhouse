# Agent prompt: migrate a module to the RBACv2 role model of DKP 1.78

Give the text below to a coding agent working in the module repository. It is written to be pasted as
is; the only thing to adjust is the module name and the path, if the agent cannot infer them.

The agent does the same work as `rbacv2-migrate-module.sh`, plus the judgement calls the script
deliberately leaves to a human — which tier a permission belongs to, and which level. If you only want
the mechanical part, run the script instead.

---

You are migrating this module's RBACv2 templates from the legacy `manage`/`use` scheme to the role
model of DKP 1.78. Work only in this repository; do not touch any cluster. Do not reformat files or
reorder anything you are not asked to change — the diff has to stay reviewable.

## Background you need

Permissions live in **capabilities**: `ClusterRole` objects with rules and one or more
`rbac.deckhouse.io/aggregate-to-<lineage>-as: <level>` labels. **Roles** are empty `ClusterRole`
objects that aggregate capabilities by those labels; a module ships capabilities only.

There are two tiers a module ships:

- **namespace tier** (`templates/rbacv2/use/`) — what a tenant does with the module's resources inside
  a namespace. Granted with a `RoleBinding`. Levels: `viewer`, `user`, `manager`, `admin`, `superadmin`.
- **system tier** (`templates/rbacv2/manage/`) — the module's own cluster-scoped resources and its
  `ModuleConfig`. Granted with a `ClusterRoleBinding` on a subsystem or system role. Levels: `viewer`,
  `manager`, `superadmin`. The subsystems are `deckhouse`, `infrastructure`, `kubernetes`,
  `networking`, `observability`, `security`, `storage`.

## What to do

For every file under `templates/rbacv2/**`:

1. Rename the object:
   - `d8:use:capability:module:<module>:<action>` → `d8:namespace-capability:<module>:<action>`
   - `d8:manage:permission:module:<module>:<action>` → `d8:system-capability:<module>:<action>`
2. Fix the labels:
   - `rbac.deckhouse.io/kind: use` or `manage` → `rbac.deckhouse.io/kind: capability`
   - add `rbac.deckhouse.io/scope: namespace` for the namespace tier, `system` for the system tier
   - add `rbac.deckhouse.io/capability: "<scope>-capability.<module>.<action>"` — it must be unique
     across the whole platform, so keep the module name in it
   - on the namespace tier, replace every `rbac.deckhouse.io/aggregate-to-<lineage>-as` label with a
     single `rbac.deckhouse.io/aggregate-to-namespace-as: <level>`, keeping the level
   - on the system tier, leave the `rbac.deckhouse.io/aggregate-to-<subsystem>-as` labels as they are,
     including several at once
   - delete `rbac.deckhouse.io/level`
   - keep `rbac.deckhouse.io/namespace: d8-<module>` on the system tier; it is what creates an
     automatic `RoleBinding` in the module's namespace, and it is only read when `scope` is `system`
     or `subsystem`
   - if the labels are templated through `helm_lib_module_labels`, change the same pairs inside the
     `dict` call
3. Add the four localized annotations, using exactly this wording (`<module>` is the module name):

   | Tier and action | `en.…/title` | `ru.…/title` | `en.…/description` | `ru.…/description` |
   |---|---|---|---|---|
   | use/view | `Module <module>: view` | `Модуль <module>: просмотр` | `Read-only access to <module> resources in a namespace.` | `Доступ только на чтение к ресурсам модуля <module> в пространстве имён.` |
   | use/edit | `Module <module>: edit` | `Модуль <module>: редактирование` | `Manage <module> resources in a namespace.` | `Управление ресурсами модуля <module> в пространстве имён.` |
   | manage/view | `Module <module>: view configuration` | `Модуль <module>: просмотр конфигурации` | `Read-only access to the <module> module configuration.` | `Доступ только на чтение к конфигурации модуля <module>.` |
   | manage/edit | `Module <module>: edit configuration` | `Модуль <module>: управление конфигурацией` | `Manage the <module> module configuration.` | `Управление конфигурацией модуля <module>.` |

## The judgement calls — report each one you make

1. **Check the tier of every rule.** A namespace capability now aggregates into namespace roles, where
   a rule on a cluster-scoped resource grants nothing. If a file under `use/` grants a cluster-scoped
   resource, move that rule into the matching file under `manage/`. Say which rules you moved and why.
2. **Check the level.** Most modules used `viewer` for `view` and `manager` for `edit`; keep what you
   find unless the module clearly means otherwise. The namespace lineage also has `user` — the level of
   a tenant who merely works in the namespace. If a namespace capability is dropped from a second
   lineage (a legacy `use` file sometimes carried `kubernetes` and `networking` together), say so.

## When you are done

- `grep -rn 'rbac.deckhouse.io/kind: \(use\|manage\)\|d8:use:capability\|d8:manage:permission\|rbac.deckhouse.io/level' templates/` returns nothing.
- `helm template . --set global.enabledModules='{}' >/dev/null` still renders.
- Every capability has: a `d8:` name matching its scope, `kind: capability`, a `scope`, a unique
  `capability` marker, at least one `aggregate-to-*-as` label, the four annotations, `rules`, and no
  `aggregationRule`.

Report as a short list: the files changed, the rules moved between tiers, the levels you kept or
changed, and anything you could not decide.
