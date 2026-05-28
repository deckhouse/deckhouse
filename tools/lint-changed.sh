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

# Run golangci-lint only in Go modules touched by the diff between $DIFF_BASE
# and HEAD. Nested modules are handled via longest-prefix matching so a change
# under dhctl/foo/ goes to the dhctl/foo module, not the parent dhctl module.
#
# Inputs (env vars):
#   DIFF_BASE          — git ref/SHA to diff against. Default: HEAD~1.
#   GOLANGCI_LINT_BIN  — golangci-lint binary path. Default: golangci-lint.
#   GOLANGCI_LINT_ARGS — extra args appended to `golangci-lint run`.

set -Eeuo pipefail

DIFF_BASE="${DIFF_BASE:-HEAD~1}"
GOLANGCI_LINT_BIN="${GOLANGCI_LINT_BIN:-golangci-lint}"
GOLANGCI_LINT_ARGS="${GOLANGCI_LINT_ARGS:---max-issues-per-linter 100 --max-same-issues 100}"

# golangci-lint must already be on PATH — we deliberately don't install it
# here (the CI tests image ships /usr/local/bin/golangci-lint). Fail fast
# with a clear hint instead of letting `golangci-lint run` error cryptically.
if ! command -v "$GOLANGCI_LINT_BIN" >/dev/null 2>&1; then
  echo "golangci-lint not found on PATH (looked for: '$GOLANGCI_LINT_BIN')." >&2
  echo "Run 'make golangci-lint' to install it locally, or set GOLANGCI_LINT_BIN." >&2
  exit 1
fi

# All module directories, repo-relative, sorted by descending path length so
# the longest prefix wins for nested modules.
mapfile -t MODULE_DIRS < <(
  find . -name go.mod -type f \
    -not -path '*/.git/*' \
    -not -path '*/node_modules/*' \
    -exec dirname {} \; \
    | sed -e 's|^\./||' \
    | awk '{ print length, $0 }' \
    | sort -k1,1nr \
    | cut -d' ' -f2-
)

if (( ${#MODULE_DIRS[@]} == 0 )); then
  echo "No go.mod files found in repo."
  exit 0
fi

# Files we care about: Go sources, module manifests, and per-module lint
# configs. A README change under dhctl/ should not trigger dhctl's linter.
mapfile -t CHANGED < <(
  git diff --name-only "$DIFF_BASE"...HEAD -- \
    '*.go' \
    'go.mod' 'go.sum' \
    '**/go.mod' '**/go.sum' \
    '.golangci.yaml' '.golangci.yml' \
    '**/.golangci.yaml' '**/.golangci.yml'
)

if (( ${#CHANGED[@]} == 0 )); then
  echo "No Go-relevant files changed between $DIFF_BASE and HEAD."
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

if (( ${#AFFECTED[@]} == 0 )); then
  echo "Changed files don't map to any Go module."
  exit 0
fi

echo "Affected Go modules (diff base: $DIFF_BASE):"
for mod in "${!AFFECTED[@]}"; do echo "  - $mod"; done

FAILED=0
for mod in "${!AFFECTED[@]}"; do
  echo ""
  echo "============================================================"
  echo "Running golangci-lint in ./${mod}"
  echo "============================================================"
  (
    cd "$mod"
    GOFLAGS="-buildvcs=false" "$GOLANGCI_LINT_BIN" run $GOLANGCI_LINT_ARGS
  ) || FAILED=1
done

exit "$FAILED"
