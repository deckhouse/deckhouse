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

set -euo pipefail

source .gitlab/scripts/api_get.sh

THRESHOLD_SECONDS=300

write_measure_env() {
{
    echo "SLOW=${SLOW}"
    echo "DURATION_HUMAN=\"${DURATION_HUMAN}\""
    echo "JOB_URL=${JOB_URL}"
    
} >> measure.env
}

# Initialize dotenv payload up front so after_script/reporting can rely on stable keys even when checks are skipped.
# Default output for all skip/not-applicable paths.
SLOW=false
DURATION_HUMAN=
JOB_URL=

JOBS_JSON="$(api_get "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/pipelines/${CI_PIPELINE_ID}/jobs?per_page=100")"

# Primary target is this pipeline's reproducibility build; regex fallback keeps metric collection resilient to naming drift.
BUILD_JOB_JSON="$(echo "${JOBS_JSON}" | jq -c '
(map(select(.name == "build-fe:reproducibility")) | sort_by(.id) | last)
// (map(select(.name | test("^build-fe")) | .) | sort_by(.id) | last)
// empty
')"

if [[ -z "${BUILD_JOB_JSON}" ]]; then
echo "No completed-successful build-fe job (conclusion=absent); slow check skipped."
write_measure_env
exit 0
fi

BUILD_JOB_STATUS="$(echo "${BUILD_JOB_JSON}" | jq -r '.status // empty')"
BUILD_JOB_STARTED_AT="$(echo "${BUILD_JOB_JSON}" | jq -r '.started_at // empty')"
BUILD_JOB_FINISHED_AT="$(echo "${BUILD_JOB_JSON}" | jq -r '.finished_at // empty')"
BUILD_JOB_NAME="$(echo "${BUILD_JOB_JSON}" | jq -r '.name // "build-fe"')"
BUILD_JOB_WEB_URL="$(echo "${BUILD_JOB_JSON}" | jq -r '.web_url // empty')"

if [[ "${BUILD_JOB_STATUS}" != "success" || -z "${BUILD_JOB_STARTED_AT}" || -z "${BUILD_JOB_FINISHED_AT}" ]]; then
echo "No completed-successful build-fe job (conclusion=${BUILD_JOB_STATUS:-absent}); slow check skipped."
write_measure_env
exit 0
fi

STARTED_TS="$(date -d "${BUILD_JOB_STARTED_AT}" +%s)"
FINISHED_TS="$(date -d "${BUILD_JOB_FINISHED_AT}" +%s)"
DUR_SEC="$((FINISHED_TS - STARTED_TS))"

if (( DUR_SEC < 0 )); then
echo "No completed-successful build-fe job (conclusion=${BUILD_JOB_STATUS}); slow check skipped."
write_measure_env
exit 0
fi

DUR_HUMAN="$((DUR_SEC / 60))m $((DUR_SEC % 60))s"
echo "Build FE job '${BUILD_JOB_NAME}' duration: ${DUR_HUMAN} (${DUR_SEC}s); threshold ${THRESHOLD_SECONDS}s"

if (( DUR_SEC > THRESHOLD_SECONDS )); then
SLOW=true
DURATION_HUMAN="${DUR_HUMAN}"
JOB_URL="${BUILD_JOB_WEB_URL}"
fi

write_measure_env
