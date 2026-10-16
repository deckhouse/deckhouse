#!/usr/bin/env bash
# Copyright 2026 Flant JSC
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Brings a module's RBACv2 templates to the role model of DKP 1.78. Run it in the module repository;
# it edits files in place and changes nothing in any cluster. See RBACV2_MODULE_MIGRATION.md for what
# the result must look like and what to check afterwards.
#
# Set subsystems in module.yaml to the module's subsystem in the DKP subsystem table first. When it
# names one subsystem of the role model, the script moves every subsystem lineage of a system capability
# to it. Otherwise it moves the lineages of the legacy subsystems by a fixed table, and reports a lineage
# module.yaml does not declare. An rbac.yaml with subsystems takes the place of module.yaml here, as it
# does for dmt.
#
# By default the legacy object is kept and the new one is put beside it, both behind a version gate,
# so one branch of the module serves clusters on either side of 1.78: the gate renders the new
# capability on DKP >= 1.78 and the legacy one below that. An external module has to do this --
# it ships one artifact for every supported platform version. --replace drops the legacy object
# instead, which is what an in-tree module wants, since it ships with the platform it targets --
# and must use, because the platform's own rbacv2 contract test reads templates/rbacv2/** as raw
# YAML and a gated file fails it.
#
# Usage:
#   rbacv2-migrate-module.sh [-n] [--replace] [PATH ...]
#
#   -n, --dry-run   print the diff instead of writing the files
#   --replace       rewrite in place, without keeping the legacy object
#   PATH            a directory to search, or a single template file (default: the current directory)
#
# --replace only decides what a fresh migration writes. A file that already carries the gate is
# left alone whatever the flag, so going from both schemes back to the new one alone means deleting
# the else branch and the helper by hand.
#
# Requires python3 and, for --dry-run, diff.

# No -u: empty arrays are expanded below, and bash 3.2 (the default on macOS) treats an empty array
# as unset.
set -eo pipefail

DRY_RUN=0
DUAL=1
PATHS=()
for arg in "$@"; do
  case "$arg" in
    -n|--dry-run) DRY_RUN=1 ;;
    --replace) DUAL=0 ;;
    -h|--help)
      cat <<'USAGE'
Brings a module's RBACv2 templates to the role model of DKP 1.78.
Run it in the module repository; it edits files in place and changes nothing in any cluster.

By default the legacy object is kept and the new one is put beside it, both behind a version gate,
so one branch serves clusters on either side of 1.78. --replace drops the legacy object instead.

Set subsystems in module.yaml to the module's subsystem in the DKP subsystem table first. When it
names one subsystem of the role model, every subsystem lineage of a system capability moves to it;
otherwise the legacy subsystems move by a fixed table. subsystems in rbac.yaml take the place of
module.yaml's.

Usage:
  rbacv2-migrate-module.sh [-n] [--replace] [PATH ...]

  -n, --dry-run   print the diff instead of writing the files
  --replace       rewrite in place, without keeping the legacy object
  PATH            a directory to search, or a single template file (default: the current directory)

--replace only decides what a fresh migration writes: an already gated file is left alone whatever
the flag, so dropping the legacy branch afterwards is a hand edit. In-tree modules must use it --
the platform's rbacv2 contract test parses templates/rbacv2/** as raw YAML and a gated file fails it.

See RBACV2_MODULE_MIGRATION.md for what the result must look like and what to check afterwards.
USAGE
      exit 0 ;;
    -*) echo "unknown option: $arg" >&2; exit 2 ;;
    *) PATHS+=("$arg") ;;
  esac
done
[[ ${#PATHS[@]} -eq 0 ]] && PATHS=(".")

command -v python3 >/dev/null || { echo "python3 is required" >&2; exit 1; }

FILES=()
for path in "${PATHS[@]}"; do
  if [[ -f "$path" ]]; then
    FILES+=("$path")
  elif [[ -d "$path" ]]; then
    while IFS= read -r file; do FILES+=("$file"); done < <(
      find "$path" -type f -name '*.yaml' -path '*/templates/rbacv2/*' | sort
    )
  else
    echo "no such file or directory: $path" >&2; exit 1
  fi
done

if [[ ${#FILES[@]} -eq 0 ]]; then
  echo "no RBACv2 templates found under: ${PATHS[*]}"
  echo "expected files at <module>/templates/rbacv2/{manage,use}/<action>.yaml"
  exit 0
fi

DRY_RUN=$DRY_RUN DUAL=$DUAL python3 - "${FILES[@]}" <<'PYTHON'
import os
import re
import subprocess
import sys
import tempfile

DRY_RUN = os.environ.get("DRY_RUN") == "1"
DUAL = os.environ.get("DUAL") == "1"

# The legacy names carry everything needed to build the new ones: the tier the object belonged to
# (use = permissions inside a namespace, manage = the module's own configuration), the module, and
# the action: view and edit, or an action of the module's own (download_snapshots). A module with a
# subsystem of its own named its objects after it instead (virtualization:
# d8:use:capability:virtualization:<action>, d8:manage:permission:subsystem:virtualization:<action>).
# A name without a segment is the module's only when its module label says so, a subsystem: name only
# when the subsystem is the module's own (own_subsystems): d8:use:capability:kubernetes:* and
# d8:manage:permission:subsystem:kubernetes:* are lineages of the platform.
LEGACY_NAME = re.compile(r"^(\s*name:\s*)(d8:(use:capability|manage:permission):(?:(module|subsystem):)?([a-z0-9-]+):([a-z0-9_]+))\s*$", re.M)

# The roles a module with a subsystem of its own shipped in the legacy scheme: namespace roles that
# gathered its use capabilities (d8:use:role:<level>:<module>, d8:use:<module>:<level>), and the
# subsystem roles (d8:manage:<subsystem>:<level>) that 1.78 names d8:subsystem:<subsystem>:<level>.
LEGACY_USE_ROLE = re.compile(r"^\s*name:\s*(d8:use:(?:role:([a-z]+):([a-z0-9-]+)|([a-z0-9-]+):([a-z]+)))\s*$", re.M)
LEGACY_MANAGE_ROLE = re.compile(r"^(\s*name:\s*)(d8:manage:([a-z0-9-]+):([a-z]+))\s*$", re.M)

NAMESPACE_LEVELS = ["viewer", "user", "manager", "admin", "superadmin"]
SYSTEM_LEVELS = ["viewer", "manager", "superadmin"]

# The subsystems of the role model, in the order of the DKP subsystem table, which puts every DKP module
# in one of them. user-authz ships the roles d8:subsystem:<subsystem>:<level> of each, so a module ships
# none of them. Only a module outside the table may declare a subsystem of its own (own_subsystems). No
# DKP module does now, and virtualization, which had one in the legacy scheme, is in cluster.
SUBSYSTEMS = ["iam", "security", "cluster", "delivery", "network", "storage", "observability", "managed-services"]

# The subsystems of the legacy scheme, each with the subsystem of the role model that took it over.
# deckhouse, infrastructure and kubernetes went into cluster, and networking is network now.
LEGACY_SUBSYSTEMS = {
    "deckhouse": "cluster",
    "infrastructure": "cluster",
    "kubernetes": "cluster",
    "networking": "network",
    "observability": "observability",
    "security": "security",
    "storage": "storage",
}

# The lineages of the platform, legacy and current. Any other rbac.deckhouse.io/aggregate-to-<x>-as
# is the subsystem of a module's own (own_subsystems) or an axis of a module (state-snapshotter's
# backup-agent), which the contract of the role model refuses as an unknown lineage: it is neither a
# level to migrate to nor one to drop, since a role of the module collects the capability by it.
LINEAGES = ["all", "namespace", "project", "system"] + SUBSYSTEMS + [s for s in LEGACY_SUBSYSTEMS if s not in SUBSYSTEMS]

SCOPE_OF_TIER = {"use": "namespace", "manage": "system"}

# The wording every migrated module in the platform repository uses. Keep it: the console shows these
# strings next to the capability, and a module that words them differently reads as an outlier.
TEXTS = {
    ("use", "view"): (
        "Module {m}: view",
        "Модуль {m}: просмотр",
        "Read-only access to {m} resources in a namespace.",
        "Доступ только на чтение к ресурсам модуля {m} в пространстве имён.",
    ),
    ("use", "edit"): (
        "Module {m}: edit",
        "Модуль {m}: редактирование",
        "Manage {m} resources in a namespace.",
        "Управление ресурсами модуля {m} в пространстве имён.",
    ),
    ("manage", "view"): (
        "Module {m}: view configuration",
        "Модуль {m}: просмотр конфигурации",
        "Read-only access to the {m} module configuration.",
        "Доступ только на чтение к конфигурации модуля {m}.",
    ),
    ("manage", "edit"): (
        "Module {m}: edit configuration",
        "Модуль {m}: управление конфигурацией",
        "Manage the {m} module configuration.",
        "Управление конфигурацией модуля {m}.",
    ),
}

# The gate the wrapped files ask. It answers on every cluster, which is the whole point:
# global.deckhouseVersion is "dev" on a development build and "unknown" if the version file is
# missing, and semverCompare raises on both, taking the whole render down with it. A release
# candidate is the other trap -- semver orders 1.78.0-rc.1 BELOW 1.78.0, so comparing the raw string
# would hand an rc build of 1.78 the legacy object. Both are avoided by comparing on major.minor
# only. The parentheses around .Values.global are the third: a chart rendered without global values
# at all would otherwise fail on a nil pointer.
#
# Anything that is not a version renders the NEW object, and that choice is about the direction of
# failure, not about which branch is likelier. The version file holds CI_COMMIT_TAG, so a build off
# any branch says "dev" -- a dev stand built from release-1.77 is indistinguishable from one built
# from main. Of the two ways to be wrong there, only one is dangerous:
#   new object on a cluster of the legacy scheme -- nothing there selects aggregate-to-namespace-as,
#     so the object is inert and the module's permissions are simply missing;
#   legacy object on a cluster of the role model -- a use capability that carries the lineage of a
#     subsystem the role model kept (observability, security, storage) is aggregated by
#     d8:subsystem:<subsystem>:<level>, which is granted through a ClusterRoleBinding, so rules meant
#     for one namespace become cluster-wide.
# One direction loses access, the other hands it out. A dev stand on a release branch below 1.78
# that needs the legacy objects flips the "true" below to "false" in that build.
GATE_HEADER = """{{- /*
  Generated by rbacv2-migrate-module.sh. Tells the RBACv2 templates of this chart which role model
  the cluster they are rendered for speaks: the model of DKP 1.78 and later, or the legacy
  manage/use one. Remove it once the module no longer supports clusters below 1.78.
*/ -}}
"""

GATE_DEFINE = """{{- define "%(module)s.rbacv2_new_scheme" -}}
  {{- $raw := (.Values.global).deckhouseVersion | default "dev" | toString -}}
  {{- $mm := regexFind "^v?[0-9]+[.][0-9]+" $raw -}}
  {{- if $mm -}}
    {{- semverCompare ">= 1.78" (printf "%%s.0" $mm) -}}
  {{- else -}}
    {{- /* "dev" or "unknown": a build off any branch says the same, so answer with the model
           whose mistake only loses access. A dev stand below 1.78 flips this to false. */ -}}
    true
  {{- end -}}
{{- end -}}
"""

GATE_MARKER = "rbacv2_new_scheme"


def gate_open(module):
    return '{{- if eq (include "%s.rbacv2_new_scheme" .) "true" }}\n' % module


def gate_body(modules):
    """The whole helper file for one chart: the header once, then a define per module."""
    return GATE_HEADER + "".join(GATE_DEFINE % {"module": m} for m in sorted(modules))


def normalised_gate(text):
    """The helper with the dev answer masked, so the one documented hand-edit compares equal."""
    return re.sub(r"(?m)^(\s*)(?:true|false)$", r"\1BOOL", text)


def chart_root(path):
    """The directory holding templates/ for this file, i.e. where the helper belongs."""
    marker = os.sep + "templates" + os.sep
    # A path given as "templates/rbacv2/use/view.yaml" carries no leading separator, so the marker
    # would not match and the helper would land inside the rbacv2 directory.
    rooted = path if path.startswith(("." + os.sep, os.sep)) else os.path.join(".", path)
    index = rooted.find(marker)
    return rooted[:index] if index != -1 else os.path.dirname(rooted)


def listed_subsystems(path, name):
    """The subsystems a file at the chart root lists, or None when the chart has no such file. Read
    without a YAML library, as the script needs none: an inline list or a block list under subsystems."""
    try:
        with open(os.path.join(chart_root(path), name), encoding="utf-8") as handle:
            text = handle.read()
    except OSError:
        return None
    inline = re.search(r"(?m)^subsystems:\s*\[([^\]]*)\]", text)
    if inline:
        names = [n.strip().strip("'\"") for n in inline.group(1).split(",")]
    else:
        block = re.search(r"(?m)^subsystems:\s*\n((?:[ \t]*-[^\n]*\n?)+)", text)
        names = [n.strip().lstrip("-").strip().strip("'\"") for n in block.group(1).splitlines()] if block else []
    return [n for n in names if n]


def declared_subsystems(path):
    """The subsystems the chart's module.yaml declares, or None when the chart has no module.yaml."""
    return listed_subsystems(path, "module.yaml")


def aggregated_subsystems(path):
    """The subsystems the system capabilities of the chart aggregate into. dmt reads them from rbac.yaml
    when it lists any, for a module that aggregates into more subsystems than module.yaml declares, and
    from module.yaml otherwise. None when the chart has neither."""
    return listed_subsystems(path, "rbac.yaml") or declared_subsystems(path)


def module_subsystems(path):
    """The subsystems the chart's module.yaml declares beyond the platform's, legacy or current: a
    subsystem of the module's own. cluster and the other subsystems of the role model are the
    platform's, so a module that moved into one of them has none."""
    return [n for n in declared_subsystems(path) or [] if n not in LINEAGES]


def platform_subsystems(path):
    """The subsystems of the role model the system capabilities of the chart aggregate into
    (aggregated_subsystems). A legacy id counts as the subsystem that took it over."""
    found = []
    for name in aggregated_subsystems(path) or []:
        subsystem = LEGACY_SUBSYSTEMS.get(name, name)
        if subsystem in SUBSYSTEMS and subsystem not in found:
            found.append(subsystem)
    return found


def own_subsystems(path, text):
    """The subsystems of the module's own its legacy objects may carry: the ones module.yaml declares
    beyond the platform's, and the one named after the module. The second outlives the first. A module
    that moved from a subsystem of its own into one of the platform (virtualization into cluster) no
    longer declares the subsystem its legacy objects are named after."""
    own = module_subsystems(path)
    for module in (module_label(text, path), chart_module(path)):
        if module and module not in LINEAGES and module not in own:
            own.append(module)
    return own


def sole_subsystem(path):
    """The one subsystem of the role model the system capabilities of the chart aggregate into, or None
    when module.yaml (or rbac.yaml) names none of them, or several."""
    platform = platform_subsystems(path)
    return platform[0] if len(platform) == 1 else None


def own_target(path, subsystem):
    """The lineage the capabilities of the module's own subsystem aggregate into in the role model. It
    is the subsystem itself while module.yaml declares it, and otherwise the one subsystem of the
    platform module.yaml declares. None when module.yaml names none of them, or several."""
    if subsystem in module_subsystems(path):
        return subsystem
    return sole_subsystem(path)


def retarget(path, text, helm, own, check=True):
    """The text with the subsystem lineages of a system capability moved to the role model. When
    module.yaml puts the module in one subsystem of the role model, every subsystem lineage goes to it,
    whichever subsystem it named before: the DKP subsystem table puts every module in one. Otherwise a
    legacy id goes to the subsystem that took it over and the module's own subsystem to own_target. A
    subsystem of the module's own that module.yaml declares stays. Two labels that land on one lineage
    merge into the first. None when an own subsystem has nowhere to go. check compares the lineages with
    module.yaml; a subsystem capability is user-authz's, aggregated across subsystems by design, and
    neither moves to one subsystem nor is compared."""
    sole = sole_subsystem(path) if check else None
    declared_own = module_subsystems(path)
    if helm:
        label = re.compile(r'(\s*)"rbac\.deckhouse\.io/aggregate-to-([a-z0-9-]+)-as"(\s+)"([a-z]+)"')
    else:
        label = re.compile(r'(?m)^([ \t]*)rbac\.deckhouse\.io/aggregate-to-([a-z0-9-]+)-as:([ \t]*)("?)([a-z]+)("?)[ \t]*(?:\n|$)')
    kept = {}
    stuck = []

    def move(m):
        lineage = m.group(2)
        level = m.group(4) if helm else m.group(5)
        if lineage in declared_own:
            target = lineage
        elif sole and (lineage in LEGACY_SUBSYSTEMS or lineage in SUBSYSTEMS or lineage in own):
            target = sole
        elif lineage in LEGACY_SUBSYSTEMS:
            target = LEGACY_SUBSYSTEMS[lineage]
        elif lineage in own:
            target = own_target(path, lineage)
            if target is None:
                stuck.append(lineage)
                return m.group(0)
        else:
            target = lineage
        if target in kept:
            if kept[target] != level:
                warn(path, f"rbac.deckhouse.io/aggregate-to-{lineage}-as: {level} also lands on the {target} lineage, which "
                           f"the capability already aggregates into as {kept[target]}; kept {kept[target]}, check which level "
                           "this capability belongs to")
            return ""
        kept[target] = level
        if helm:
            return f'{m.group(1)}"rbac.deckhouse.io/aggregate-to-{target}-as"{m.group(3)}"{level}"'
        end = "\n" if m.group(0).endswith("\n") else ""
        return f"{m.group(1)}rbac.deckhouse.io/aggregate-to-{target}-as:{m.group(3)}{m.group(4)}{level}{m.group(6)}{end}"

    moved = label.sub(move, text)
    if stuck:
        warn(path, f"aggregates into {', '.join(sorted(set(stuck)))}, the subsystem of the module's own, and module.yaml "
                   "declares neither it nor exactly one subsystem of the platform to move it to; set subsystems in "
                   "module.yaml to the module's subsystem in the DKP subsystem table, then rerun")
        return None
    if check:
        check_declared(path, moved, helm)
    return moved


declared_checked = set()   # chart roots whose module.yaml was already checked for legacy subsystem ids


def check_declared(path, text, helm):
    """Report a subsystem lineage of a system capability that module.yaml (or rbac.yaml) does not
    declare, and, once per chart, a legacy subsystem id in module.yaml."""
    declared = declared_subsystems(path)
    if declared is None:
        return
    root = chart_root(path)
    if root not in declared_checked:
        declared_checked.add(root)
        legacy = [n for n in declared if n in LEGACY_SUBSYSTEMS and n not in SUBSYSTEMS]
        if legacy:
            warn(os.path.join(root, "module.yaml"),
                 "subsystems names " + ", ".join(f"{n}, which the role model calls {LEGACY_SUBSYSTEMS[n]}" for n in legacy)
                 + "; set it to the module's subsystem in the DKP subsystem table")
    known = set(platform_subsystems(path)) | set(module_subsystems(path))
    override = listed_subsystems(path, "rbac.yaml")
    source, listed = ("rbac.yaml", override) if override else ("module.yaml", declared)
    for lineage in dict.fromkeys(lineage for lineage, _ in aggregation_labels(text, helm)):
        if lineage in SUBSYSTEMS and lineage not in known:
            said = f"subsystems: [{', '.join(listed)}]" if listed else "it declares no subsystems"
            warn(path, f"aggregates into the {lineage} subsystem, which {source} does not declare ({said}); check the "
                       f"module's subsystem in the DKP subsystem table, and keep the lineage only if the capability "
                       f"belongs to {lineage} as well")


def chart_module(path):
    """The module helm_lib_module_labels names the objects of this chart after: .Chart.Name, that is the
    name of Chart.yaml, then the name of module.yaml, then the chart directory without its NNN- prefix."""
    root = chart_root(path)
    for name in ("Chart.yaml", "module.yaml"):
        try:
            with open(os.path.join(root, name), encoding="utf-8") as handle:
                m = re.search(r'(?m)^name:\s*"?([a-z0-9-]+)"?\s*$', handle.read())
        except OSError:
            continue
        if m:
            return m.group(1)
    return re.sub(r"^[0-9]+-", "", os.path.basename(os.path.normpath(root)))


def module_label(text, path):
    """The module label of a template: a plain label, a "module" pair of a dict, or, for a template that
    writes its labels through helm_lib_module_labels, the chart's module that helper injects at render
    time. None when there is none."""
    m = re.search(r'(?m)^\s*module:\s*"?([a-z0-9-]+)"?\s*$', text) or re.search(r'"module"\s+"([a-z0-9-]+)"', text)
    if m:
        return m.group(1)
    if "helm_lib_module_labels" in text:
        return chart_module(path)
    return None


warnings = []
migrated = []
retargeted = []     # objects of the role model whose subsystem lineages were moved where they are
skipped = []
gates = {}          # chart root -> set of module names whose defines the helper must carry
pending = []        # (path, text) to write once every helper is in place


def warn(path, message):
    warnings.append(f"{path}: {message}")


def aggregation_labels(text, helm):
    """Every rbac.deckhouse.io/aggregate-to-<lineage>-as label with its level, in file order."""
    if helm:
        pattern = r'"rbac\.deckhouse\.io/aggregate-to-([a-z0-9-]+)-as"\s+"([a-z]+)"'
    else:
        pattern = r"rbac\.deckhouse\.io/aggregate-to-([a-z0-9-]+)-as:\s*\"?([a-z]+)\"?"
    return re.findall(pattern, text)


def generic_texts(action):
    """Texts for an action outside view/edit, from its name: the script cannot know what it means."""
    words = action.replace("_", " ")
    return (
        "Module {m}: " + words,
        "Модуль {m}: " + words,
        "The " + words + " capability of the {m} module.",
        "Capability " + words + " модуля {m}.",
    )


LEGACY_ONLY = ""   # migrate() returns it for an object 1.78 has no counterpart of: the gate keeps its legacy branch
LEGACY_KEEPS = " and only the legacy branch keeps it" if DUAL else ""
gate_module = {}  # path -> module the gate of the file is named after, for objects that are not capabilities
in_place = set()  # paths of objects of the role model rewritten where they are, without a gate around them


def alias_gate(module, gated):
    """What an alias in templates/rbacv2-compat/ needs while the legacy branch keeps the old name."""
    if not gated:
        return ""
    return (f". The legacy branch keeps the role under that name below the version the gate checks, so render "
            f"the alias only behind {{{{- if eq (include \"{module}.rbacv2_new_scheme\" .) \"true\" }}}}")


def use_alias(module, level, gated):
    """How to keep d8:use:role:<level>:<module> or d8:use:<module>:superadmin, a namespace role of the module,
    as an alias: delegatable like d8:namespace:<level>, and refused for a new binding in a project."""
    below = ", ".join(NAMESPACE_LEVELS[:NAMESPACE_LEVELS.index(level) + 1])
    return (f"If bindings use the name, keep it as an alias in templates/rbacv2-compat/: selectors by module: {module}, "
            f"rbac.deckhouse.io/kind: capability and rbac.deckhouse.io/aggregate-to-namespace-as for each of "
            f"{below}, with the labels rbac.deckhouse.io/deprecated: \"true\" and rbac.deckhouse.io/delegatable: "
            f"\"true\" and the annotations rbac.deckhouse.io/deprecated-replaced-by: d8:namespace:{level} and "
            "rbac.deckhouse.io/disabled-for-direct-use-in-projects: \"true\", so a RoleBinding to it in a project "
            "namespace keeps working and a new ProjectRoleBinding to it is refused" + alias_gate(module, gated))


def manage_alias(module, subsystem, level, target, gated):
    """How to keep d8:manage:<subsystem>:<level>, a role of the module's own subsystem, as an alias once
    module.yaml puts the module in target: no wider than the role was, and not delegatable."""
    below = ", ".join(SYSTEM_LEVELS[:SYSTEM_LEVELS.index(level) + 1])
    return (f"If bindings use d8:manage:{subsystem}:{level}, keep that name as an alias in templates/rbacv2-compat/ "
            f"that grants no more than the role did: selectors by module: {module}, rbac.deckhouse.io/kind: "
            f"capability, rbac.deckhouse.io/scope: system and rbac.deckhouse.io/aggregate-to-{target}-as for each "
            f"of {below}, with the label rbac.deckhouse.io/deprecated: \"true\" and the annotation "
            f"rbac.deckhouse.io/deprecated-replaced-by: d8:subsystem:{target}:{level}, and without "
            "rbac.deckhouse.io/delegatable" + alias_gate(module, gated))


def migrate_role(path, text):
    """A legacy role of a module with a subsystem of its own, migrated or kept for the legacy branch
    only; None when the text holds none."""
    if not re.search(r"^aggregationRule:", text, re.M):
        return None
    module = module_label(text, path)
    use = LEGACY_USE_ROLE.search(text)
    if use and re.search(r'kind"?[:\s]+"?use\b', text):
        level, owner = (use.group(2), use.group(3)) if use.group(2) else (use.group(5), use.group(4))
        if owner == module and level in NAMESPACE_LEVELS:
            gate_module[path] = module
            warn(path, f"{use.group(1)} gathered the module's use capabilities; in the role model d8:namespace:{level} "
                       f"gathers the migrated ones, so the role has no counterpart{LEGACY_KEEPS}. "
                       + use_alias(module, level, DUAL))
            return LEGACY_ONLY
    manage = LEGACY_MANAGE_ROLE.search(text)
    if manage and re.search(r'kind"?[:\s]+"?manage\b', text):
        subsystem, level = manage.group(3), manage.group(4)
        if subsystem in own_subsystems(path, text) and level in SYSTEM_LEVELS:
            target = own_target(path, subsystem)
            if target is None:
                warn(path, f"{manage.group(2)} is a role of {subsystem}, the subsystem of the module's own, and module.yaml "
                           "declares neither it nor exactly one subsystem of the platform to move it to; set subsystems in "
                           "module.yaml to the module's subsystem in the DKP subsystem table, then rerun")
                return None
            if target != subsystem:
                gate_module[path] = module or chart_module(path)
                warn(path, f"{manage.group(2)} was a role of {subsystem}, the subsystem of the module's own, and module.yaml "
                           f"puts the module in {target}, whose role d8:subsystem:{target}:{level} user-authz ships; the role "
                           f"has no counterpart{LEGACY_KEEPS}. "
                           + manage_alias(gate_module[path], subsystem, level, target, DUAL))
                return LEGACY_ONLY
            if "helm_lib_module_labels" in text:
                warn(path, f"{manage.group(2)} writes its labels through helm_lib_module_labels; migrate it to "
                           f"d8:subsystem:{subsystem}:{level} by hand (see RBACV2_MODULE_MIGRATION.md, "
                           "\"A subsystem of the module's own\")")
                return None
            # The gate is named after the chart's module, so every gated file of the chart asks the same
            # define; a role without a module label still gets one.
            gate_module[path] = module or chart_module(path)
            if not module:
                warn(path, f"{manage.group(2)} carries no module label; the gate is named after the chart ({gate_module[path]}), "
                           f"and d8:subsystem:{subsystem}:{level} needs module: {gate_module[path]} like the rest of the module")
            return subsystem_role(path, text, manage, subsystem, level)
    return None


def subsystem_role(path, text, match, subsystem, level):
    """d8:manage:<subsystem>:<level> of the module's own subsystem as the 1.78 subsystem role
    d8:subsystem:<subsystem>:<level>, shaped as user-authz's own: kind role and scope subsystem, the
    subsystem label, the ladder of its lineage kept, aggregate-to-all-as moved to aggregate-to-system-as,
    and the selector by its lineage alone."""
    head, sep, rule = text.partition("\naggregationRule:")
    head = re.sub(r"^(\s*)rbac\.deckhouse\.io/kind:\s*\"?manage\"?\s*$",
                  rf"\g<1>rbac.deckhouse.io/kind: role\n\g<1>rbac.deckhouse.io/scope: subsystem", head, count=1, flags=re.M)
    head = re.sub(r"^\s*rbac\.deckhouse\.io/level:\s*\"?[a-z]+\"?\s*$\n", "", head, flags=re.M)
    head = re.sub(r"^(\s*)rbac\.deckhouse\.io/aggregate-to-all-as:", r"\g<1>rbac.deckhouse.io/aggregate-to-system-as:", head, flags=re.M)
    if not re.search(r"^\s*rbac\.deckhouse\.io/subsystem:", head, re.M):
        head = re.sub(r"^(\s*)rbac\.deckhouse\.io/scope: subsystem$", rf"\g<0>\n\g<1>rbac.deckhouse.io/subsystem: {subsystem}", head, count=1, flags=re.M)
    if not re.search(r"^\s*rbac\.deckhouse\.io/use-role:", head, re.M):
        warn(path, f"d8:subsystem:{subsystem}:{level} needs rbac.deckhouse.io/use-role, the namespace role its holders get "
                   "in the module's namespaces; add it")
    # The capabilities of 1.78 carry kind capability: a selector that still asks for kind manage matches none.
    rule = re.sub(r"^\s*rbac\.deckhouse\.io/kind:\s*\"?manage\"?\s*$\n", "", rule, flags=re.M)
    title = f"Subsystem {subsystem}: {level}"
    annotations = ""
    if "meta.deckhouse.io/title" not in text:
        annotations = (
            f'\n  annotations:\n'
            f'    en.meta.deckhouse.io/title: "{title}"\n'
            f'    ru.meta.deckhouse.io/title: "Подсистема {subsystem}: {level}"\n'
            f'    en.meta.deckhouse.io/description: "The {level} role of the {subsystem} subsystem."\n'
            f'    ru.meta.deckhouse.io/description: "Роль {level} подсистемы {subsystem}."'
        )
        warn(path, f"the titles and descriptions of d8:subsystem:{subsystem}:{level} are written from its name; reword them "
                   "in both languages: the console shows them next to the role")
    head = LEGACY_MANAGE_ROLE.sub(lambda m: f"{m.group(1)}d8:subsystem:{subsystem}:{level}{annotations}", head, count=1)
    return head + sep + rule


def retarget_migrated(path, text, gated=False):
    """An object of the role model, migrated before the subsystems of the role model were settled, with
    its subsystem lineages moved to them. None when there is nothing to move, or when the object is a
    human's to change and a warning says so. gated says the object is the new branch of a gated file."""
    helm = "helm_lib_module_labels" in text
    own = own_subsystems(path, text)
    scope = re.search(r'rbac\.deckhouse\.io/scope"?[:\s]+"?([a-z]+)', text)
    scope = scope.group(1) if scope else ""
    if re.search(r'kind"?[:\s]+"?capability\b', text):
        if scope in ("system", "subsystem"):
            moved = retarget(path, text, helm, own, check=scope == "system")
            return moved if moved is not None and moved != text else None
        stray = [lineage for lineage, _ in aggregation_labels(text, helm)
                 if lineage in LEGACY_SUBSYSTEMS or lineage in SUBSYSTEMS or lineage in own]
        if stray and scope == "namespace":
            warn(path, f"a namespace capability carries rbac.deckhouse.io/aggregate-to-{stray[0]}-as, the lineage of a "
                       "subsystem; drop it, a namespace capability aggregates into the namespace lineage only")
        return None
    subsystem = re.search(r'rbac\.deckhouse\.io/subsystem"?[:\s]+"?([a-z0-9-]+)', text)
    subsystem = subsystem.group(1) if subsystem else ""
    if re.search(r'kind"?[:\s]+"?role\b', text) and scope == "subsystem" and subsystem in own:
        target = own_target(path, subsystem)
        role = re.search(rf"(?m)^\s*name:\s*(d8:subsystem:{re.escape(subsystem)}:([a-z]+))\s*$", text)
        if target not in (None, subsystem) and role and role.group(2) in SYSTEM_LEVELS:
            # An earlier run wrote the role from d8:manage:<subsystem>:<level>, the name the module released.
            # The role it wrote is the one to drop; the released name is the one bindings may use.
            module = module_label(text, path) or chart_module(path)
            if gated:
                drop = ("Empty the new branch, and keep the gate and the else branch, which still renders "
                        f"d8:manage:{subsystem}:{role.group(2)} below the version the gate checks. ")
            else:
                drop = "Delete the file. "
            warn(path, f"{role.group(1)} is a role of {subsystem}, the subsystem of the module's own, and module.yaml puts the "
                       f"module in {target}, whose role d8:subsystem:{target}:{role.group(2)} user-authz ships, so the role has "
                       f"no counterpart. {drop}" + manage_alias(module, subsystem, role.group(2), target, gated))
    return None


def advise_legacy_role(path, legacy):
    """Report the alias the legacy branch of a gated file needs when that branch holds a role of the module
    that has no counterpart in the role model and the new branch is empty. A fresh run reports the same
    from migrate_role."""
    body = legacy.split("\n", 1)[1] if "\n" in legacy else ""
    body = body.rsplit("{{- end }}", 1)[0]
    if not re.search(r"^aggregationRule:", body, re.M):
        return
    module = module_label(body, path) or chart_module(path)
    use = LEGACY_USE_ROLE.search(body)
    if use and re.search(r'kind"?[:\s]+"?use\b', body):
        level, owner = (use.group(2), use.group(3)) if use.group(2) else (use.group(5), use.group(4))
        if owner == module and level in NAMESPACE_LEVELS:
            warn(path, f"the legacy branch holds {use.group(1)}, a namespace role of the module that has no counterpart "
                       "in the role model. " + use_alias(module, level, True))
        return
    manage = LEGACY_MANAGE_ROLE.search(body)
    if manage and re.search(r'kind"?[:\s]+"?manage\b', body):
        subsystem, level = manage.group(3), manage.group(4)
        if subsystem not in own_subsystems(path, body) or level not in SYSTEM_LEVELS:
            return
        target = own_target(path, subsystem)
        if target not in (None, subsystem):
            warn(path, f"the legacy branch holds {manage.group(2)}, a role of {subsystem}, the subsystem of the module's own, "
                       f"and module.yaml puts the module in {target}, so the role has no counterpart. "
                       + manage_alias(module, subsystem, level, target, True))


# The two branches of a gated file, as wrap() writes them. The new branch ends at the first "{{- else }}"
# at the start of a line; a template conditional inside the rules is indented with them.
GATED = re.compile(r'(?s)\A(.*?\{\{- if eq \(include "[a-z0-9-]+\.rbacv2_new_scheme" \.\) "true" \}\}\n)(.*?\n)(\{\{- else \}\}\n.*)\Z')


def retarget_gated(path, text):
    """A gated file with the subsystem lineages of its new branch moved to the role model, the legacy
    branch untouched. None when there is nothing to move."""
    m = GATED.match(text)
    if m is None:
        return None
    if not m.group(2).strip():
        advise_legacy_role(path, m.group(3))
        return None
    moved = retarget_migrated(path, m.group(2), gated=True)
    return None if moved is None else m.group(1) + moved + m.group(3)


def migrate(path, text):
    match = LEGACY_NAME.search(text)
    reason = "not a legacy module capability"
    if match is not None:
        segment, name = match.group(4), match.group(5)
        if segment is None and module_label(text, path) != name:
            match = None  # d8:use:capability:kubernetes:* and the like: a lineage of the platform, not a module
        elif segment == "subsystem" and name not in own_subsystems(path, text):
            match = None
            reason = "a subsystem of the platform, not the module's own"
    if match is None:
        role = migrate_role(path, text)
        if role is not None:
            return role
        if "rbac.deckhouse.io/kind" in text and re.search(r'kind"?[:\s]+"?(capability|role)', text):
            moved = retarget_migrated(path, text)
            if moved is not None:
                in_place.add(path)
                return moved
            skipped.append(f"{path}: already migrated")
            return None
        # A use capability left alone that still carries the label of the module's own subsystem pours
        # its namespace rules into the cluster-wide subsystem role of 1.78, which selects by that label
        # alone. Say so rather than leave it among the files that are fine.
        if re.search(r'kind"?[:\s]+"?use\b', text):
            for subsystem in own_subsystems(path, text):
                if re.search(rf'rbac\.deckhouse\.io/aggregate-to-{re.escape(subsystem)}-as', text):
                    if own_target(path, subsystem) == subsystem:
                        warn(path, f"a use capability the script does not recognise carries rbac.deckhouse.io/aggregate-to-{subsystem}-as: "
                                   f"in 1.78 d8:subsystem:{subsystem}:<level> selects by that label alone and would grant its "
                                   "rules cluster-wide; migrate it by hand (see RBACV2_MODULE_MIGRATION.md, "
                                   "\"A subsystem of the module's own\")")
                    else:
                        warn(path, f"a use capability the script does not recognise carries rbac.deckhouse.io/aggregate-to-{subsystem}-as, "
                                   "the lineage of the module's own subsystem; migrate it by hand (see RBACV2_MODULE_MIGRATION.md, "
                                   "\"A subsystem of the module's own\")")
                    break
        skipped.append(f"{path}: {reason}, left alone")
        return None

    tier = "use" if match.group(3) == "use:capability" else "manage"
    module, action = match.group(5), match.group(6)
    own = own_subsystems(path, text)
    gate_module[path] = module
    scope = SCOPE_OF_TIER[tier]
    marker = f"{scope}-capability.{module}.{action}"
    new_name = f"d8:{scope}-capability:{module}:{action}"
    if len(marker) > 63:
        room = 63 - len(marker) + len(action)
        warn(path, f"the marker {marker} is {len(marker)} characters, and a label value holds 63: the API server refuses "
                   f"the object and the release of the module fails. Shorten the action to at most {room} characters "
                   "in the new name and marker before deploying")

    # A legacy name on a role that only aggregates (no rules of its own) is a role of the module, and
    # the 1.78 model has no place for one: it is not a capability to rename.
    if re.search(r"^aggregationRule:", text, re.M):
        warn(path, "an aggregating role, not a capability: 1.78 has no role of a module in the model. Ship it as an "
                   "ordinary ClusterRole outside the model (no rbac.deckhouse.io/kind, named d8:<module>:<name>, "
                   "aggregationRule by a label of the module) and keep this name as an alias in "
                   "templates/rbacv2-compat/; see RBACV2_MODULE_MIGRATION.md")
        skipped.append(f"{path}: an aggregating role, left alone")
        return None

    helm = "helm_lib_module_labels" in text
    labels = aggregation_labels(text, helm)
    levels = [(lineage, level) for lineage, level in labels if lineage in LINEAGES]
    # The module's own subsystem is a lineage of 1.78 for a system capability. For a use capability it
    # was the ladder of the module's namespace roles (virtualization's use and manage objects shared
    # it, told apart by kind); 1.78 subsystem roles select by the label alone, so a namespace
    # capability that kept it would pour its rules into the cluster-wide subsystem role. Its level
    # moves to the namespace lineage, and the label goes.
    own_levels = [(lineage, level) for lineage, level in labels if lineage in own]
    if tier == "manage":
        levels += own_levels
    elif not levels:
        levels = [(lineage, level) for lineage, level in own_levels if level in NAMESPACE_LEVELS]
    axes = [lineage for lineage, _ in labels if lineage not in LINEAGES and lineage not in own]
    # The lineages a use capability's labels are rewritten from: the platform's and the module's own.
    use_lineage = "(?:" + "|".join(LINEAGES + own) + ")"
    if not levels:
        if axes:
            warn(path, f"aggregates only into {', '.join(axes)}, which the role model does not know: a capability also "
                       "aggregates into a level of the platform, or the contract refuses it -- add one, then rerun")
        else:
            warn(path, "no rbac.deckhouse.io/aggregate-to-*-as label: the capability would land in no role")
        return None
    for axis in axes:
        warn(path, f"rbac.deckhouse.io/aggregate-to-{axis}-as is not a lineage of the role model, and the 1.78 contract "
                   f"refuses it; it is kept so the role that collects it keeps the capability. Move it to a label of "
                   f"the module (for example {module}.deckhouse.io/aggregate-to-{axis}: \"true\") on every capability "
                   f"that role collects, and select by it in that role")
    if tier == "use" and len({level for _, level in levels}) > 1:
        warn(path, f"the aggregation labels disagree on the level ({levels}); "
                   "collapsed into the first one, check which level this capability belongs to")

    level = levels[0][1]

    if helm:
        # The labels live inside a (dict "key" "value" ...) call, so they are rewritten as pairs.
        text = re.sub(r'"rbac\.deckhouse\.io/kind"\s+"(use|manage)"',
                      f'"rbac.deckhouse.io/kind" "capability" '
                      f'"rbac.deckhouse.io/scope" "{scope}"', text, count=1)
        text = re.sub(r"\(dict ", f'(dict "rbac.deckhouse.io/capability" "{marker}" ', text, count=1)
        text = re.sub(r'\s*"rbac\.deckhouse\.io/level"\s+"[a-z]+"', "", text)
        if tier == "use":
            text = re.sub(rf'"rbac\.deckhouse\.io/aggregate-to-{use_lineage}-as"\s+"[a-z]+"',
                          f'"rbac.deckhouse.io/aggregate-to-namespace-as" "{level}"', text, count=1)
            text = re.sub(rf'\s*"rbac\.deckhouse\.io/aggregate-to-(?!namespace-as){use_lineage}-as"\s+"[a-z]+"', "", text)
    else:
        indent = re.search(r"^(\s*)rbac\.deckhouse\.io/kind:", text, re.M)
        pad = indent.group(1) if indent else "    "
        text = re.sub(r"^(\s*)rbac\.deckhouse\.io/kind:\s*\"?(use|manage)\"?\s*$",
                      f"{pad}rbac.deckhouse.io/kind: capability\n"
                      f'{pad}rbac.deckhouse.io/capability: "{marker}"\n'
                      f"{pad}rbac.deckhouse.io/scope: {scope}", text, count=1, flags=re.M)
        text = re.sub(r"^\s*rbac\.deckhouse\.io/level:\s*\"?[a-z]+\"?\s*$\n", "", text, flags=re.M)
        if tier == "use":
            text = re.sub(rf"^(\s*)rbac\.deckhouse\.io/aggregate-to-{use_lineage}-as:\s*\"?[a-z]+\"?\s*$",
                          rf"\g<1>rbac.deckhouse.io/aggregate-to-namespace-as: {level}",
                          text, count=1, flags=re.M)
            text = re.sub(rf"^\s*rbac\.deckhouse\.io/aggregate-to-(?!namespace-as){use_lineage}-as:\s*\"?[a-z]+\"?\s*$\n",
                          "", text, flags=re.M)

    if tier == "manage":
        # No role of the role model aggregates a legacy subsystem lineage, and a module that moved out of
        # a subsystem of its own aggregates into the subsystem module.yaml declares now.
        text = retarget(path, text, helm, own)
        if text is None:
            return None

    if tier == "use" and re.search(r"rbac\.deckhouse\.io/namespace", text):
        warn(path, "the rbac.deckhouse.io/namespace label is only read on system and subsystem "
                   "capabilities; on a namespace one it does nothing")

    texts = TEXTS.get((tier, action))
    if texts is None:
        texts = generic_texts(action)
        if "meta.deckhouse.io/title" not in text:
            warn(path, f"the titles and descriptions of {new_name} are written from its name; reword them in both "
                       "languages: the console shows them next to the capability")
    en_title, ru_title, en_desc, ru_desc = (t.format(m=module) for t in texts)
    annotations = (
        f'  annotations:\n'
        f'    en.meta.deckhouse.io/title: "{en_title}"\n'
        f'    ru.meta.deckhouse.io/title: "{ru_title}"\n'
        f'    en.meta.deckhouse.io/description: "{en_desc}"\n'
        f'    ru.meta.deckhouse.io/description: "{ru_desc}"\n'
    )
    if "meta.deckhouse.io/title" in text:
        annotations = ""

    text = LEGACY_NAME.sub(lambda m: f"{m.group(1)}{new_name}\n{annotations}".rstrip("\n"), text, count=1)
    return text


def wrap(path, legacy, new, module):
    """Put the new object and the legacy one in one file, behind the version gate."""
    if not module:
        sys.exit(f"{path}: no module to name the version gate after; this is a bug of the script")
    gates.setdefault(chart_root(path), set()).add(module)
    return (
        gate_open(module)
        + new.rstrip("\n") + "\n"
        + "{{- else }}\n"
        + legacy.rstrip("\n") + "\n"
        + "{{- end }}\n"
    )


def unified_diff(path, before, after):
    with tempfile.NamedTemporaryFile("w", suffix=".yaml") as old, \
         tempfile.NamedTemporaryFile("w", suffix=".yaml") as new:
        old.write(before), new.write(after)
        old.flush(), new.flush()
        result = subprocess.run(["diff", "-u", "--label", path, "--label", path, old.name, new.name],
                                capture_output=True, text=True)
        return result.stdout


for path in sys.argv[1:]:
    with open(path, encoding="utf-8") as handle:
        before = handle.read()
    if GATE_MARKER in before:
        # Still remember the chart: the helper the gate calls may have been deleted by hand, and a
        # file that asks for a template nobody defines fails the render of the whole chart.
        for gated in re.findall(r'include "([a-z0-9-]+)\.rbacv2_new_scheme"', before):
            gates.setdefault(chart_root(path), set()).add(gated)
        # The new branch of an earlier run may still name a legacy subsystem; the legacy branch must.
        after = retarget_gated(path, before)
        if after is None:
            skipped.append(f"{path}: already carries the version gate")
            continue
        retargeted.append(path)
        if DRY_RUN:
            sys.stdout.write(unified_diff(path, before, after))
        else:
            pending.append((path, after))
        continue
    # One file, one object. migrate() renames only the first occurrence, so a second object would
    # travel into the branch a 1.78 cluster reads while still carrying its legacy name.
    if len(LEGACY_NAME.findall(before)) > 1:
        warn(path, "several legacy objects in one file; migrate it by hand, "
                   "one ClusterRole per file, then rerun")
        skipped.append(f"{path}: several legacy objects in one file")
        continue
    after = migrate(path, before)
    if after is None or after == before:
        continue
    if after == LEGACY_ONLY and not DUAL:
        warn(path, "has no counterpart in the 1.78 model and --replace keeps no legacy branch: delete the file, and "
                   "keep its name as an alias in templates/rbacv2-compat/ if bindings use it")
        skipped.append(f"{path}: a legacy role with no 1.78 counterpart, left for you to delete")
        continue
    if path in in_place:
        retargeted.append(path)
    else:
        if DUAL:
            after = wrap(path, before, after, gate_module[path])
        migrated.append(path)
    if DRY_RUN:
        sys.stdout.write(unified_diff(path, before, after))
    else:
        pending.append((path, after))

# Helpers first: a template that asks for a define nobody wrote fails the render of the whole
# chart, so the tree must never be left with the gate in place and the helper missing.
gate_files = []
kept_gates = []
for root in sorted(gates):
    gate_path = os.path.join(root, "templates", "_rbacv2_compat.tpl")
    body = gate_body(gates[root])
    if os.path.exists(gate_path):
        with open(gate_path, encoding="utf-8") as handle:
            current = handle.read()
        if current == body:
            continue
        # The one hand-edit this script documents is flipping the dev answer to false. Keep it.
        if normalised_gate(current) == normalised_gate(body):
            kept_gates.append(gate_path)
            continue
    gate_files.append(gate_path)
    if not DRY_RUN:
        with open(gate_path, "w", encoding="utf-8") as handle:
            handle.write(body)

for path, text in pending:
    with open(path, "w", encoding="utf-8") as handle:
        handle.write(text)

print()
print(f"migrated: {len(migrated)}" + (" (dry run, nothing written)" if DRY_RUN else ""))
for path in migrated:
    print(f"  {path}")
if retargeted:
    print(f"moved to the subsystems of the role model: {len(retargeted)}" + (" (dry run, nothing written)" if DRY_RUN else ""))
    for path in retargeted:
        print(f"  {path}")
if gate_files:
    print(f"version gate: {len(gate_files)}" + (" (dry run, nothing written)" if DRY_RUN else ""))
    for path in gate_files:
        print(f"  {path}")
if kept_gates:
    print(f"version gate kept as edited: {len(kept_gates)}")
    for path in kept_gates:
        print(f"  {path}")
if skipped:
    print(f"untouched: {len(skipped)}")
    for line in skipped:
        print(f"  {line}")
if warnings:
    print(f"needs a human: {len(warnings)}")
    for line in warnings:
        print(f"  {line}")

if (migrated or retargeted) and not DRY_RUN:
    print()
    print("Now check by hand:")
    print("  1. The capability marker matches the module name, and no other module in the cluster uses it.")
    print("  2. A use capability grants only namespaced resources; anything cluster-scoped belongs to a")
    print("     system capability instead (cluster-scoped rules in a namespace role grant nothing).")
    print("  3. The level (viewer/user/manager/admin) is still the one this module intends.")
    print("  4. Every system capability aggregates into the module's subsystem in the DKP subsystem table,")
    print("     the one subsystems in module.yaml names.")
    print("  5. The generated titles and descriptions read correctly for this module.")
    print("  6. helm template still renders: the labels of a templated file live inside a dict call.")
    if DUAL:
        print("  7. Both branches render: helm template with global.deckhouseVersion set to a version")
        print("     below 1.78 gives the legacy object, at or above it the new one, and \"dev\" gives")
        print("     the new one.")
        print("  8. The module declares no requirement that already excludes clusters below 1.78 --")
        print("     if module.yaml requires a newer platform, drop the legacy branch with --replace.")
        print("  9. Template tests that look a ClusterRole up by name set global.deckhouseVersion:")
        print("     a harness that leaves it unset renders the new scheme, so an assertion on a")
        print("     legacy name fails.")
PYTHON
