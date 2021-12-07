#!/bin/bash

# Copyright 2021 Flant JSC
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

# This script has just one purpose:
# Fetch inputs for validation script:
#   - title and description of merge request
#   - diff content

set -Eeo pipefail

# Create tmp dir and set traps to clean it after execution.
TMPDIR=$(mktemp -d ./tmp.curl.XXXXXX)
cleanup() {
  rm -rf $TMPDIR
}
trap '(exit 130)' INT
trap '(exit 143)' TERM
trap 'rc=$?; cleanup; exit $rc' EXIT

# Helper to download diff.
function download_diff() {
  curl --silent -L -f "${DIFF_URL}"
}

# Check prerequisites: validation script, DIFF_URL

VALIDATION_SCRIPT=$1
if [[ ! -f $VALIDATION_SCRIPT ]]; then
  echo "Validation script '${VALIDATION_SCRIPT}' is not found."
  exit 1
fi
if [[ ! -x $VALIDATION_SCRIPT ]]; then
  echo "Validation script '${VALIDATION_SCRIPT}' is not executable."
  exit 1
fi

if [[ -z $DIFF_URL ]]; then
  echo "No diff download url provided: DIFF_URL env is empty."
  exit 1
fi

# Download diff to validate.
echo "Fetch changes ..."
diffFile="$TMPDIR/validate.this.diff"
if ! download_diff > "${diffFile}" ; then
  echo "Error downloading diff from '${DIFF_URL}'. Exit code $?."
  exit 1
fi

affected=$(grep -c '^diff --git a' "${diffFile}" || true)
removed=$(grep -v '^--- a/' "${diffFile}" | grep -c '^-' || true)
added=$(grep -v '^+++ b/' "${diffFile}" | grep -c '^+' || true)
echo "  diff: ${affected} files affected, ${added} lines added, ${removed} lines removed."

# Run validation script using preinstalled Go.
if ! "${VALIDATION_SCRIPT}" "${diffFile}" ; then
  echo -e "\nFix the problem or use '${SKIP_LABEL_NAME}' PR label to skip."
  exit 1
fi
