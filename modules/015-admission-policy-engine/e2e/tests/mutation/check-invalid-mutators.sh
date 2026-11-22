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

# Every mutator the ValidatingAdmissionPolicies of the module forbid is rejected at apply time
# with the policy message. Server-side dry run, nothing is created.
set -Eeuo pipefail
source "../common/scripts/lib.sh"

failed=()

# file | expected fragment of the message (fixed string)
table=(
  'manifests/invalid/assign-metadata-location.yaml|spec.location cannot target the "metadata" field'
  'manifests/invalid/assignmetadata-spec-location.yaml|spec.location must be exactly metadata.labels.<key> or metadata.annotations.<key>'
  'manifests/invalid/assignmetadata-non-string.yaml|spec.parameters.assign.value must be a plain string for AssignMetadata'
  'manifests/invalid/assign-without-kinds.yaml|spec.match.kinds is required and must not be empty'
)

for row in "${table[@]}"; do
  IFS='|' read -r file message <<<"${row}"
  rc=0
  out="$(kubectl create --dry-run=server -f "${file}" 2>&1)" || rc=$?
  if (( rc != 0 )) && grep -qF -- "${message}" <<<"${out}"; then
    log "OK ${file}: ${out}"
  else
    log "UNEXPECTED ${file} (exit code ${rc}): ${out}"
    failed+=("${file}")
  fi
done

(( ${#failed[@]} == 0 )) || fail "mutators admitted or rejected with another message: ${failed[*]}"
