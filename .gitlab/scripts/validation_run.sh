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

# This script has two purposes:
# 1. Run a validation script if specific label is set on merge request.
# 2. Fetch inputs for validation script:
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

# Helper to request Gitlab API.
function request_gitlab_api() {
  curl --silent -f -H "PRIVATE-TOKEN: ${GITLAB_API_TOKEN}" "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/${1}"
}

# Check prerequisites: label name, validation script, API token

if [[ -z $SKIP_LABEL_NAME ]]; then
  echo "Label name is not provided."
  exit 1
fi

VALIDATION_SCRIPT=$1
if [[ ! -f $VALIDATION_SCRIPT ]]; then
  echo "Validation script '${VALIDATION_SCRIPT}' is not found."
  exit 1
fi
if [[ ! -x $VALIDATION_SCRIPT ]]; then
  echo "Validation script '${VALIDATION_SCRIPT}' is not executable."
  exit 1
fi

GITLAB_API_TOKEN="$2"
if [[ -z $GITLAB_API_TOKEN ]]; then
  echo "No API access token provided."
  exit 1
fi

# TODO CI_MERGE_REQUEST_IID should be available in Gitlab 11.6, but it is absent in 14.3.0.
# TODO Remove this workaround when CI_MERGE_REQUEST_IID become available.
echo "Fetch MR IID ..."
CI_MERGE_REQUEST_IID=$(request_gitlab_api "merge_requests?state=opened" | jq -r --arg c ${CI_COMMIT_SHA} '.[]|select(.sha == $c) | .iid')
if [[ "$CI_MERGE_REQUEST_IID" == "" ]]; then
  echo "No MR found for commit sha: ${CI_COMMIT_SHA}"
  exit 0
fi
# END workaround

# Fetch diff for merge request into file.
echo "Fetch changes for MR!${CI_MERGE_REQUEST_IID} ..."
changesUrl="merge_requests/${CI_MERGE_REQUEST_IID}/changes?access_raw_diffs=true"
if ! request_gitlab_api "${changesUrl}" > "$TMPDIR/mr.info.json" ; then
  echo "Error requesting MR changes."
  exit 1
fi

echo -n "MR labels: " ; jq --arg label_name "${SKIP_LABEL_NAME}" '.labels' < "$TMPDIR/mr.info.json"

# Check if validation should be skipped according to MR labels.
if jq -r '.labels[]' "$TMPDIR/mr.info.json" | grep "^${SKIP_LABEL_NAME}$" >/dev/null ; then
  echo "MR has '$SKIP_LABEL_NAME' label, skip validation..."
  exit 0
fi

# Prepare MR title and description for validation script.
export VALIDATE_TITLE=$(jq -r '.title' < "$TMPDIR/mr.info.json")
export VALIDATE_DESCRIPTION=$(jq -r '.description' < "$TMPDIR/mr.info.json")

# Prepare diff file for validation script.
jq '.changes[]
  | "diff --git a/"+ .old_path + " b/"+ .new_path ,
    "new file mode "+ .b_mode ,
    "--- " + (if .new_file then "/dev/null" else "a/"+.old_path end) ,
    "+++ " + (if .deleted_file then "/dev/null" else "b/"+ .new_path end) ,
    .diff|split("\n")[]
' -r < "$TMPDIR/mr.info.json" > "$TMPDIR/mr.diff"

# Run validation script in docker container.
if ! docker run --rm \
   -v $(pwd):/src \
   -w /src \
   -e VALIDATE_TITLE \
   -e VALIDATE_DESCRIPTION \
   --entrypoint bash \
   "${BASE_GOLANG_16_BUSTER}" \
   "${VALIDATION_SCRIPT}" "${TMPDIR}/mr.diff"
then
  echo -e "\nFix the problem or use '${SKIP_LABEL_NAME}' MR label to skip."
  exit 1
fi
