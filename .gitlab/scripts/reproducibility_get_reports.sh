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

if [[ -z "${BASELINE_PIPELINE_ID:-}" ]]; then
echo "ERROR: BASELINE_PIPELINE_ID is empty. It must be provided from build-fe:reproducibility dotenv artifact." >&2
exit 1
fi

# include_retried=true + latest successful selection avoids stale artifacts from superseded retried jobs.
BASELINE_JOBS_JSON="$(api_get "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/pipelines/${BASELINE_PIPELINE_ID}/jobs?scope[]=success&include_retried=true&per_page=100")"

# Prefer canonical main build artifact; fallback below supports historical/alternative build_fe:* names.
BASELINE_BUILD_JOB_ID="$(echo "${BASELINE_JOBS_JSON}" | jq -r '
map(select(.name == "build_fe:main" and (.artifacts_file.filename != null)))
| sort_by(.id)
| last
| .id // empty
')"

if [[ -z "${BASELINE_BUILD_JOB_ID}" ]]; then
BASELINE_BUILD_JOB_ID="$(echo "${BASELINE_JOBS_JSON}" | jq -r '
    map(select((.name | test("^build_fe:")) and (.artifacts_file.filename != null)))
    | sort_by(.id)
    | last
    | .id // empty
')"
fi

if [[ -z "${BASELINE_BUILD_JOB_ID}" ]]; then
echo "ERROR: Could not find successful baseline build_fe:* job with artifacts in pipeline ${BASELINE_PIPELINE_ID}." >&2
exit 1
fi

curl -sfS --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
--output baseline-images_tags_werf.json \
"${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/jobs/${BASELINE_BUILD_JOB_ID}/artifacts/images_tags_werf.json"

if [[ ! -f images_tags_werf.json ]]; then
echo "ERROR: images_tags_werf.json from build-fe:reproducibility artifacts was not found in compare-digests workspace." >&2
exit 1
fi

# build-fe:reproducibility artifact is current run output in workspace; rename copy to explicit rerun role for comparison script.
cp images_tags_werf.json rerun-images_tags_werf.json

# Comparison failures are converted to dotenv flag so notification flow can proceed with diagnostics.
python3 .github/scripts/python/reproducibility_check.py baseline-images_tags_werf.json rerun-images_tags_werf.json || echo "COMPARE_RESULT=failure" >> measure.env
