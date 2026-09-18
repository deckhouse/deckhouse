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
  echo "expected files at <module>/templates/rbacv2/{manage,use}/{view,edit}.yaml"
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
# the action.
LEGACY_NAME = re.compile(r"^(\s*name:\s*)(d8:(use:capability|manage:permission):module:([a-z0-9-]+):(view|edit))\s*$", re.M)

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
#   new object on a 1.77 cluster -- nothing there selects aggregate-to-namespace-as, so the object
#     is inert and the module's permissions are simply missing;
#   legacy object on a 1.78 cluster -- d8:subsystem:kubernetes:<level> still aggregates
#     aggregate-to-kubernetes-as and is granted through a ClusterRoleBinding, so rules meant for one
#     namespace become cluster-wide.
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


warnings = []
migrated = []
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


def migrate(path, text):
    match = LEGACY_NAME.search(text)
    if match is None:
        if "rbac.deckhouse.io/kind" in text and re.search(r'kind"?[:\s]+"?(capability|role)', text):
            skipped.append(f"{path}: already migrated")
        else:
            skipped.append(f"{path}: not a legacy module capability, left alone")
        return None

    tier = "use" if match.group(3) == "use:capability" else "manage"
    module, action = match.group(4), match.group(5)
    scope = SCOPE_OF_TIER[tier]
    marker = f"{scope}-capability.{module}.{action}"
    new_name = f"d8:{scope}-capability:{module}:{action}"

    helm = "helm_lib_module_labels" in text
    levels = aggregation_labels(text, helm)
    if not levels:
        warn(path, "no rbac.deckhouse.io/aggregate-to-*-as label: the capability would land in no role")
        return None
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
            text = re.sub(r'"rbac\.deckhouse\.io/aggregate-to-[a-z0-9-]+-as"\s+"[a-z]+"',
                          f'"rbac.deckhouse.io/aggregate-to-namespace-as" "{level}"', text, count=1)
            text = re.sub(r'\s*"rbac\.deckhouse\.io/aggregate-to-(?!namespace)[a-z0-9-]+-as"\s+"[a-z]+"', "", text)
    else:
        indent = re.search(r"^(\s*)rbac\.deckhouse\.io/kind:", text, re.M)
        pad = indent.group(1) if indent else "    "
        text = re.sub(r"^(\s*)rbac\.deckhouse\.io/kind:\s*\"?(use|manage)\"?\s*$",
                      f"{pad}rbac.deckhouse.io/kind: capability\n"
                      f'{pad}rbac.deckhouse.io/capability: "{marker}"\n'
                      f"{pad}rbac.deckhouse.io/scope: {scope}", text, count=1, flags=re.M)
        text = re.sub(r"^\s*rbac\.deckhouse\.io/level:\s*\"?[a-z]+\"?\s*$\n", "", text, flags=re.M)
        if tier == "use":
            text = re.sub(r"^(\s*)rbac\.deckhouse\.io/aggregate-to-[a-z0-9-]+-as:\s*\"?[a-z]+\"?\s*$",
                          rf"\g<1>rbac.deckhouse.io/aggregate-to-namespace-as: {level}",
                          text, count=1, flags=re.M)
            text = re.sub(r"^\s*rbac\.deckhouse\.io/aggregate-to-(?!namespace)[a-z0-9-]+-as:\s*\"?[a-z]+\"?\s*$\n",
                          "", text, flags=re.M)

    if tier == "use" and re.search(r"rbac\.deckhouse\.io/namespace", text):
        warn(path, "the rbac.deckhouse.io/namespace label is only read on system and subsystem "
                   "capabilities; on a namespace one it does nothing")

    en_title, ru_title, en_desc, ru_desc = (t.format(m=module) for t in TEXTS[(tier, action)])
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
        skipped.append(f"{path}: already carries the version gate")
        # Still remember the chart: the helper the gate calls may have been deleted by hand, and a
        # file that asks for a template nobody defines fails the render of the whole chart.
        for gated in re.findall(r'include "([a-z0-9-]+)\.rbacv2_new_scheme"', before):
            gates.setdefault(chart_root(path), set()).add(gated)
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
    if DUAL:
        module_match = LEGACY_NAME.search(before)
        after = wrap(path, before, after, module_match.group(4))
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

if migrated and not DRY_RUN:
    print()
    print("Now check by hand:")
    print("  1. The capability marker matches the module name, and no other module in the cluster uses it.")
    print("  2. A use capability grants only namespaced resources; anything cluster-scoped belongs to a")
    print("     system capability instead (cluster-scoped rules in a namespace role grant nothing).")
    print("  3. The level (viewer/user/manager/admin) is still the one this module intends.")
    print("  4. The generated titles and descriptions read correctly for this module.")
    print("  5. helm template still renders: the labels of a templated file live inside a dict call.")
    if DUAL:
        print("  6. Both branches render: helm template with global.deckhouseVersion set to a version")
        print("     below 1.78 gives the legacy object, at or above it the new one, and \"dev\" gives")
        print("     the new one.")
        print("  7. The module declares no requirement that already excludes clusters below 1.78 --")
        print("     if module.yaml requires a newer platform, drop the legacy branch with --replace.")
        print("  8. Template tests that look a ClusterRole up by name set global.deckhouseVersion:")
        print("     a harness that leaves it unset renders the new scheme, so an assertion on a")
        print("     legacy name fails.")
PYTHON
