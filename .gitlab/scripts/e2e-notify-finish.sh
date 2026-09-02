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

# Runs from the create job's `after_script`, so CI_JOB_STATUS reflects the
# create_cluster result. Appends the e2e outcome to the comment posted by
# e2e-ensure-build (NOTE_ID/MERGE_REQUEST_IID, via dotenv) and sets a
# final e2e-framework::<success|failed> label, removing the other one first.
#
# after_script runs in a separate shell from script/before_script and its
# exit code doesn't affect the job result — this script is best-effort and
# never fails the job it's attached to.

# COMMENT_TOKEN arrives via dotenv (needs:artifacts) from e2e-ensure-build,
# which fetched it once from vault — this job doesn't fetch its own secret.
COMMENT_TOKEN="${COMMENT_TOKEN:-}"
NOTE_ID="${NOTE_ID:-}"
MERGE_REQUEST_IID="${MERGE_REQUEST_IID:-}"

if [[ -z "${NOTE_ID}" || -z "${MERGE_REQUEST_IID}" || -z "${COMMENT_TOKEN}" ]]; then
  echo "NOTE_ID/MERGE_REQUEST_IID/COMMENT_TOKEN not available; skipping e2e finish notification"
  exit 0
fi

api() {
  local method="$1"
  local url="$2"
  shift 2
  curl -sS --fail-with-body \
    --request "${method}" \
    --header "PRIVATE-TOKEN: ${COMMENT_TOKEN}" \
    "$@" \
    "${url}"
}

if [[ "${CI_JOB_STATUS:-}" == "success" ]]; then
  STATUS_LABEL="success"
  STATUS_LINE="✅ e2e finished: success — [job](${CI_JOB_URL:-})"
else
  STATUS_LABEL="failed"
  STATUS_LINE="❌ e2e finished: failed (status=${CI_JOB_STATUS:-unknown}) — [job](${CI_JOB_URL:-})"
fi

note_url="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests/${MERGE_REQUEST_IID}/notes/${NOTE_ID}"

current_body="$(api GET "${note_url}" | jq -r '.body // empty')"
if [[ -z "${current_body}" ]]; then
  echo "Could not fetch comment ${NOTE_ID} on MR ${MERGE_REQUEST_IID}; skipping update" >&2
else
  new_body="$(printf '%s\n\n%s' "${current_body}" "${STATUS_LINE}")"
  if api PUT "${note_url}" --data-urlencode "body=${new_body}" >/dev/null; then
    echo "Updated comment ${NOTE_ID} on MR ${MERGE_REQUEST_IID}"
  else
    echo "Failed to update comment ${NOTE_ID} on MR ${MERGE_REQUEST_IID}" >&2
  fi
fi

OTHER_LABEL="failed"
[[ "${STATUS_LABEL}" == "failed" ]] && OTHER_LABEL="success"

mr_url="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests/${MERGE_REQUEST_IID}"
if api PUT "${mr_url}" \
  --data-urlencode "remove_labels=e2e-framework::${OTHER_LABEL}" \
  --data-urlencode "add_labels=e2e-framework::${STATUS_LABEL}" >/dev/null; then
  echo "Set label e2e-framework::${STATUS_LABEL} on MR ${MERGE_REQUEST_IID}"
else
  echo "Failed to set label on MR ${MERGE_REQUEST_IID}" >&2
fi

exit 0
