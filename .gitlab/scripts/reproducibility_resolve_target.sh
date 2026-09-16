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

# Resolve target SHA and baseline pipeline:
# - main mode: schedule, or manual run without REPRODUCIBILITY_COMMIT_SHA
# - dev mode:  manual run with REPRODUCIBILITY_COMMIT_SHA
INPUT_SHA="${REPRODUCIBILITY_COMMIT_SHA:-}"
INPUT_SHA="$(echo "${INPUT_SHA}" | xargs)"
DEFAULT_BRANCH="${CI_DEFAULT_BRANCH:-main}"

if [[ "${CI_PIPELINE_SOURCE}" == "schedule" || -z "${INPUT_SHA}" ]]; then
BASELINE_KIND="main"
echo "Resolving baseline in main mode (source=${CI_PIPELINE_SOURCE}, ref=${DEFAULT_BRANCH})"

# Main mode baseline selection is bounded to recent history to keep API usage predictable on schedule runs.
DEFAULT_BRANCH_REF="refs/remotes/origin/${DEFAULT_BRANCH}"
if ! git fetch --no-tags --depth=50 origin "+refs/heads/${DEFAULT_BRANCH}:${DEFAULT_BRANCH_REF}"; then
    echo "ERROR: Failed to fetch default branch '${DEFAULT_BRANCH}' from origin." >&2
    exit 1
fi

COMMITS_JSON="$(git log --format='{"id":"%H"}' -n 10 "${DEFAULT_BRANCH_REF}" | jq -sc '.')"

if [[ "$(echo "${COMMITS_JSON}" | jq 'length')" -eq 0 ]]; then
    echo "ERROR: No commits returned for default branch '${DEFAULT_BRANCH}'." >&2
    exit 1
fi

RESOLVED_SHA=""
BASELINE_PIPELINE_ID=""
BASELINE_PIPELINE_URL=""

# Prefer the newest commit whose main build already passed; downstream compare assumes available successful artifacts.
for CANDIDATE_SHA in $(echo "${COMMITS_JSON}" | jq -r '.[].id'); do
    PIPELINES_JSON="$(api_get "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/repository/commits/${CANDIDATE_SHA}" "private")"
    PIPELINE_ID="$(echo "${PIPELINES_JSON}" | jq -r '.last_pipeline.id // empty')"

    JOB_STATUS="absent"

    if [[ -n "${PIPELINE_ID}" ]]; then
    PIPELINE_URL="$(echo "${PIPELINES_JSON}" | jq -r '.last_pipeline.web_url // empty')"
    JOBS_JSON="$(api_get "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/pipelines/${PIPELINE_ID}/jobs?include_retried=true&per_page=100" "private")"
    TARGET_JOB_JSON="$(echo "${JOBS_JSON}" | jq -c 'map(select(.name == "build_fe:main")) | sort_by(.id) | last // empty')"

    if [[ -n "${TARGET_JOB_JSON}" ]]; then
        JOB_STATUS="$(echo "${TARGET_JOB_JSON}" | jq -r '.status // "absent"')"
    fi

    if [[ "${JOB_STATUS}" == "success" ]]; then
        RESOLVED_SHA="${CANDIDATE_SHA}"
        BASELINE_PIPELINE_ID="${PIPELINE_ID}"
        BASELINE_PIPELINE_URL="${PIPELINE_URL}"
        break
    fi
    fi

    echo "Skipping commit ${CANDIDATE_SHA}: build-fe:main status=${JOB_STATUS}"
done

if [[ -z "${RESOLVED_SHA}" || -z "${BASELINE_PIPELINE_ID}" ]]; then
    echo "ERROR: No suitable baseline found in the 10 newest commits of '${DEFAULT_BRANCH}' (requires build-fe:main status=success)." >&2
    exit 1
fi
else
BASELINE_KIND="dev"
RESOLVED_SHA="${INPUT_SHA}"
echo "Resolving baseline in dev mode for SHA=${RESOLVED_SHA}"

# Dev mode accepts explicit SHA, but still requires an existing successful pipeline as a baseline source of artifacts.
# Validate commit exists in repository.
api_get "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/repository/commits/${RESOLVED_SHA}" "private" >/dev/null

PIPELINES_JSON="$(api_get "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/pipelines?sha=${RESOLVED_SHA}&status=success&order_by=id&sort=desc&per_page=1")"
if [[ "$(echo "${PIPELINES_JSON}" | jq 'length')" -eq 0 ]]; then
    echo "ERROR: No successful pipeline found for SHA ${RESOLVED_SHA}. Baseline is required." >&2
    exit 1
fi

BASELINE_PIPELINE_ID="$(echo "${PIPELINES_JSON}" | jq -r '.[0].id')"
BASELINE_PIPELINE_URL="$(echo "${PIPELINES_JSON}" | jq -r '.[0].web_url // empty')"
fi

if [[ -z "${BASELINE_PIPELINE_URL}" ]]; then
BASELINE_PIPELINE_URL="${CI_PROJECT_URL}/-/pipelines/${BASELINE_PIPELINE_ID}"
fi

# Checkout must happen here and later in build-fe:reproducibility before_script to keep workspace SHA aligned across jobs.
# Ensure target commit is available locally, then checkout before .build_fe script.
if ! git cat-file -e "${RESOLVED_SHA}^{commit}" 2>/dev/null; then
git fetch --no-tags origin "${RESOLVED_SHA}" || git fetch --no-tags origin
fi
git checkout --detach "${RESOLVED_SHA}"

# Export for remaining commands in this job.
export RESOLVED_SHA
export BASELINE_PIPELINE_ID
export BASELINE_PIPELINE_URL
export BASELINE_KIND
export CI_COMMIT_SHA="${RESOLVED_SHA}"     

# Persist resolved baseline values for after_script (runs in a separate shell).
{
echo "BASELINE_PIPELINE_ID=${BASELINE_PIPELINE_ID}"
echo "BASELINE_PIPELINE_URL=${BASELINE_PIPELINE_URL}"
echo "BASELINE_KIND=${BASELINE_KIND}"
echo "RESOLVED_SHA=${RESOLVED_SHA}"
} > reproducibility.env
# Diagnostic: which commit werf will actually build from.
# RESOLVED_SHA is the workflow run's commit (immutable, runner-injected);
# git HEAD is what actions/checkout left in the work tree (may differ
# for runs that override `ref`, e.g. reproducibility-check.yml).
echo "==== git context ===="
echo "RESOLVED_SHA env:      ${RESOLVED_SHA}"
echo "Baseline kind:         ${BASELINE_KIND}"
echo "Baseline pipeline:     ${BASELINE_PIPELINE_ID} (${BASELINE_PIPELINE_URL})"
echo "git HEAD (full):       $(git rev-parse HEAD)"
echo "git HEAD (short):      $(git rev-parse --short HEAD)"
git log -1 --pretty='format:  commit:  %H%n  author:  %an <%ae>%n  date:    %ad%n  subject: %s'
echo
echo "====================="