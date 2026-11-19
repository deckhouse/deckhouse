#!/usr/bin/env bash
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

# Print the Go modules touched by the diff between $DIFF_BASE and HEAD, one
# repo-relative directory per line ("." for the root module). Only files that
# match the git pathspecs given as arguments count as changes. Nested modules
# are handled via longest-prefix matching so a change under dhctl/foo/ goes to
# the dhctl/foo module, not the parent dhctl module.
#
# Usage: changed-go-modules.sh <pathspec>...
#
# Inputs (env vars):
#   DIFF_BASE — git ref/SHA to diff against. Default: HEAD~1.

set -Eeuo pipefail

DIFF_BASE="${DIFF_BASE:-HEAD~1}"

if (( $# == 0 )); then
  echo "usage: $0 <pathspec>..." >&2
  exit 2
fi

# All module directories, repo-relative, sorted by descending path length so
# the longest prefix wins for nested modules.
mapfile -t MODULE_DIRS < <(
  find . -name go.mod -type f \
    -not -path '*/.git/*' \
    -not -path '*/node_modules/*' \
    -not -path './tools/*' \
    -exec dirname {} \; \
    | sed -e 's|^\./||' \
    | awk '{ print length, $0 }' \
    | sort -k1,1nr \
    | cut -d' ' -f2-
)

if (( ${#MODULE_DIRS[@]} == 0 )); then
  echo "No go.mod files found in repo." >&2
  exit 0
fi

# Not `< <(...)`: a failing git diff (e.g. DIFF_BASE outside a shallow clone)
# must fail the script, not look like "nothing changed".
changed="$(git diff --name-only "$DIFF_BASE"...HEAD -- "$@")"
CHANGED=()
if [[ -n "$changed" ]]; then
  mapfile -t CHANGED <<< "$changed"
fi

if (( ${#CHANGED[@]} == 0 )); then
  echo "No Go-relevant files changed between $DIFF_BASE and HEAD." >&2
  exit 0
fi

# Match each changed file to its enclosing module (longest prefix wins).
declare -A AFFECTED=()
for file in "${CHANGED[@]}"; do
  for mod in "${MODULE_DIRS[@]}"; do
    if [[ "$mod" == "." ]]; then
      # Root module — matches anything not already claimed by a deeper module
      # (loop iterates deepest-first, so a deeper match would have hit first).
      AFFECTED["."]=1
      break
    fi
    if [[ "$file" == "$mod/"* ]]; then
      AFFECTED["$mod"]=1
      break
    fi
  done
done

for mod in "${!AFFECTED[@]}"; do echo "$mod"; done | sort
