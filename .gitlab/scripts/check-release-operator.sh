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

#
# Self-review gate for deploying a release to early-access, stable or
# rock-solid: the person who triggers the deploy must not be the same
# person who created the release, i.e. who pushed the release tag that
# started this pipeline.
#
# Required env: FOX_TOKEN, CI_API_V4_URL, CI_PROJECT_ID, CI_PIPELINE_ID,
#   GITLAB_USER_LOGIN, RELEASE_CHANNEL.

set -Euo pipefail

RESPONSE="$(curl -sf --header "PRIVATE-TOKEN: ${FOX_TOKEN}" \
  "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/pipelines/${CI_PIPELINE_ID}")"

PIPELINE_CREATOR="$(echo "${RESPONSE}" | jq -r '.user.username // empty')"

if [[ -z "${PIPELINE_CREATOR}" ]]; then
  echo "Unable to determine who created the release tag pipeline (${CI_PIPELINE_ID}). Failing closed." >&2
  exit 1
fi

echo "Release tag '${CI_COMMIT_TAG}' pipeline was created by: ${PIPELINE_CREATOR}"
echo "Deploy to '${RELEASE_CHANNEL}' is being approved by: ${GITLAB_USER_LOGIN}"

if [[ "${PIPELINE_CREATOR}" == "${GITLAB_USER_LOGIN}" ]]; then
  echo "Deploying to the '${RELEASE_CHANNEL}' release channel must be approved by someone other than the release author (${PIPELINE_CREATOR})." >&2
  exit 1
fi

echo "OK: deploy operator differs from the release author."
