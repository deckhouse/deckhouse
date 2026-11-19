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
# and HEAD. The modules are found by changed-go-modules.sh.
#
# Inputs (env vars):
#   DIFF_BASE          — git ref/SHA to diff against. Default: HEAD~1.
#   GOLANGCI_LINT_BIN  — golangci-lint binary path. Default: golangci-lint.
#   GOLANGCI_LINT_ARGS — extra args appended to `golangci-lint run`.

set -Eeuo pipefail

DIFF_BASE="${DIFF_BASE:-HEAD~1}"
GOLANGCI_LINT_BIN="${GOLANGCI_LINT_BIN:-golangci-lint}"
# --new-from-merge-base mirrors the "$DIFF_BASE...HEAD" file selection below: a module
# is linted whole, so without it a PR inherits every pre-existing issue of the module
# it touches instead of only the ones its own lines introduce.
GOLANGCI_LINT_ARGS="${GOLANGCI_LINT_ARGS:---max-issues-per-linter 100 --max-same-issues 100 --new-from-merge-base $DIFF_BASE}"

# golangci-lint must already be on PATH — we deliberately don't install it
# here (the CI tests image ships /usr/local/bin/golangci-lint). Fail fast
# with a clear hint instead of letting `golangci-lint run` error cryptically.
if ! command -v "$GOLANGCI_LINT_BIN" >/dev/null 2>&1; then
  echo "golangci-lint not found on PATH (looked for: '$GOLANGCI_LINT_BIN')." >&2
  echo "Run 'make golangci-lint' to install it locally, or set GOLANGCI_LINT_BIN." >&2
  exit 1
fi

# Files we care about: Go sources, module manifests, and per-module lint
# configs. A README change under dhctl/ should not trigger dhctl's linter.
mapfile -t AFFECTED < <(
  DIFF_BASE="$DIFF_BASE" bash "$(dirname "${BASH_SOURCE[0]}")/changed-go-modules.sh" \
    '*.go' \
    'go.mod' 'go.sum' \
    '**/go.mod' '**/go.sum' \
    '.golangci.yaml' '.golangci.yml' \
    '**/.golangci.yaml' '**/.golangci.yml'
)

if (( ${#AFFECTED[@]} == 0 )); then
  echo "No Go modules to lint."
  exit 0
fi

echo "Affected Go modules (diff base: $DIFF_BASE):"
for mod in "${AFFECTED[@]}"; do echo "  - $mod"; done

FAILED=0
for mod in "${AFFECTED[@]}"; do
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
