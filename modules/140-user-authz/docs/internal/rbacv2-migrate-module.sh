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
# Rewrites a module's RBACv2 templates from the legacy manage/use scheme to the role model of
# DKP 1.78. Run it in the module repository; it edits files in place and changes nothing in any
# cluster. See RBACV2_MODULE_MIGRATION.md for what the result must look like and what to check
# afterwards.
#
# Usage:
#   rbacv2-migrate-module.sh [-n] [PATH ...]
#
#   -n, --dry-run   print the diff instead of writing the files
#   PATH            a directory to search, or a single template file (default: the current directory)
#
# Requires python3 and, for --dry-run, diff.

# No -u: empty arrays are expanded below, and bash 3.2 (the default on macOS) treats an empty array
# as unset.
set -eo pipefail

DRY_RUN=0
PATHS=()
for arg in "$@"; do
  case "$arg" in
    -n|--dry-run) DRY_RUN=1 ;;
    -h|--help)
      cat <<'USAGE'
Rewrites a module's RBACv2 templates from the legacy manage/use scheme to the role model of DKP 1.78.
Run it in the module repository; it edits files in place and changes nothing in any cluster.

Usage:
  rbacv2-migrate-module.sh [-n] [PATH ...]

  -n, --dry-run   print the diff instead of writing the files
  PATH            a directory to search, or a single template file (default: the current directory)

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

DRY_RUN=$DRY_RUN python3 - "${FILES[@]}" <<'PYTHON'
import os
import re
import subprocess
import sys
import tempfile

DRY_RUN = os.environ.get("DRY_RUN") == "1"

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

warnings = []
migrated = []
skipped = []


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
    after = migrate(path, before)
    if after is None or after == before:
        continue
    migrated.append(path)
    if DRY_RUN:
        sys.stdout.write(unified_diff(path, before, after))
    else:
        with open(path, "w", encoding="utf-8") as handle:
            handle.write(after)

print()
print(f"migrated: {len(migrated)}" + (" (dry run, nothing written)" if DRY_RUN else ""))
for path in migrated:
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
PYTHON
