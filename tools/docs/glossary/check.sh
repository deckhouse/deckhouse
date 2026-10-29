#!/bin/bash

# Copyright 2026 Flant JSC
#
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

# Checks the documentation glossary (docs/documentation/_glossary):
#   - term ids (file names) are latin kebab-case;
#   - every term has both <id>.md (en) and <id>_RU.md (ru);
#   - front matter has non-empty title and description and the right lang;
#   - every glossary.html#<anchor> link in the documentation points to a known term.
#
# Usage: tools/docs/glossary/check.sh [REPO_ROOT]

set -euo pipefail

ROOT="${1:-$(cd "$(dirname "$(realpath "$0")")/../../.." && pwd)}"
cd "${ROOT}"

GLOSSARY_DIR="docs/documentation/_glossary"
ID_RE='^[a-z0-9]+(-[a-z0-9]+)*$'
ANCHOR_RE='glossary\.html#[^\s\[\]()"'"'"'<>`]*'
SCAN_ROOTS=(docs/documentation modules ee)
SCAN_GLOBS=(
  'docs/documentation/pages/**'
  'docs/documentation/_glossary/**'
  'modules/**/docs/**'
  'modules/**/crds/**'
  'modules/**/openapi/**'
  'ee/**/modules/**/docs/**'
  'ee/**/modules/**/crds/**'
  'ee/**/modules/**/openapi/**'
  '!docs/documentation/pages/drafts/**'
  '!**/vendor/**'
)

errors=0

fail() {
  echo "ERROR: $*" >&2
  errors=$((errors + 1))
}

for tool in rg yq; do
  if ! command -v "${tool}" >/dev/null 2>&1; then
    echo "ERROR: '${tool}' is required but not found in PATH" >&2
    exit 1
  fi
done
if ! yq --version 2>&1 | grep -q mikefarah; then
  echo "ERROR: yq v4 (https://github.com/mikefarah/yq) is required, found: $(yq --version 2>&1)" >&2
  exit 1
fi

# 1. Term files: ids, front matter.
declare -A en_ids=() ru_ids=()
en_files=0
ru_files=0

check_front_matter() {
  local file="$1" expected_lang="$2"
  local fields title_len desc_len lang

  if [[ "$(head -n 1 "${file}")" != "---" ]]; then
    fail "${file}: no front matter"
    return
  fi
  if ! fields="$(yq --front-matter=extract '(.title // "" | length), (.description // "" | length), (.lang // "")' "${file}" 2>&1)"; then
    fail "${file}: cannot parse front matter: ${fields}"
    return
  fi
  { read -r title_len; read -r desc_len; read -r lang; } <<<"${fields}"

  [[ "${title_len}" != "0" ]] || fail "${file}: front matter 'title' is missing or empty"
  [[ "${desc_len}" != "0" ]] || fail "${file}: front matter 'description' is missing or empty"
  [[ "${lang}" == "${expected_lang}" ]] || fail "${file}: front matter 'lang' must be '${expected_lang}', got '${lang}'"
}

if [[ -d "${GLOSSARY_DIR}" ]]; then
  while IFS= read -r file; do
    name="$(basename "${file}" .md)"
    if [[ "${name}" == *_RU ]]; then
      id="${name%_RU}"
      lang="ru"
    else
      id="${name}"
      lang="en"
    fi

    if [[ ! "${id}" =~ ${ID_RE} ]]; then
      fail "${file}: invalid term id '${id}' (expected latin kebab-case: ^[a-z0-9]+(-[a-z0-9]+)*$)"
      continue
    fi

    check_front_matter "${file}" "${lang}"
    if [[ "${lang}" == "ru" ]]; then
      ru_ids["${id}"]=1
      ru_files=$((ru_files + 1))
    else
      en_ids["${id}"]=1
      en_files=$((en_files + 1))
    fi
  done < <(find "${GLOSSARY_DIR}" -maxdepth 1 -type f -name '*.md' | sort)
fi

# 2. Parity: every id must exist in both languages.
declare -A all_ids=()
for id in "${!en_ids[@]}" "${!ru_ids[@]}"; do
  all_ids["${id}"]=1
done
for id in $(printf '%s\n' "${!all_ids[@]}" | sort); do
  [[ -n "${en_ids[${id}]:-}" ]] || fail "term '${id}': missing ${GLOSSARY_DIR}/${id}.md"
  [[ -n "${ru_ids[${id}]:-}" ]] || fail "term '${id}': missing ${GLOSSARY_DIR}/${id}_RU.md"
done

# 3. Anchors: every glossary.html#<anchor> must be a known term id.
existing_roots=()
for root in "${SCAN_ROOTS[@]}"; do
  [[ -d "${root}" ]] && existing_roots+=("${root}")
done

glob_args=()
for glob in "${SCAN_GLOBS[@]}"; do
  glob_args+=(-g "${glob}")
done

matches=""
if [[ ${#existing_roots[@]} -gt 0 ]]; then
  rg_status=0
  matches="$(rg -o -n --no-heading --color never --sort path \
    --type-add 'docs:*.md' --type-add 'docs:*.yaml' --type-add 'docs:*.yml' -t docs \
    "${glob_args[@]}" "${ANCHOR_RE}" "${existing_roots[@]}")" || rg_status=$?
  if [[ ${rg_status} -gt 1 ]]; then
    echo "ERROR: rg failed with exit code ${rg_status}" >&2
    exit 1
  fi
fi

anchor_count=0
anchor_files=0
if [[ -n "${matches}" ]]; then
  anchor_count="$(printf '%s\n' "${matches}" | wc -l)"
  anchor_files="$(printf '%s\n' "${matches}" | cut -d: -f1 | sort -u | wc -l)"

  if [[ ! -d "${GLOSSARY_DIR}" ]]; then
    fail "${GLOSSARY_DIR} does not exist but ${anchor_count} glossary anchor(s) are referenced in the documentation"
  fi

  while IFS= read -r match; do
    location="${match%%:glossary.html#*}"
    anchor="${match##*glossary.html#}"
    if [[ -z "${anchor}" ]]; then
      fail "${location}: empty glossary anchor"
    elif [[ -z "${all_ids[${anchor}]:-}" ]]; then
      fail "${location}: unknown glossary anchor '#${anchor}'"
    fi
  done < <(printf '%s\n' "${matches}" | sort -u)
fi

if [[ ${errors} -gt 0 ]]; then
  echo "Glossary check failed: ${errors} error(s)." >&2
  exit 1
fi

echo "Glossary check passed: ${#all_ids[@]} terms (${en_files} en + ${ru_files} ru files), ${anchor_count} anchor references in ${anchor_files} files."
