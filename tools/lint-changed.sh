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

echo "Not implemented for this version"
