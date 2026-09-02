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

# Posts/updates the e2e comment on the linked merge request. Three modes
# (first arg):
#
#   start <mr_iid>  - from e2e-ensure-build. Posts the initial "started"
#                     comment and prints the created note id on stdout
#                     (all other output goes to stderr) so the caller can
#                     capture it and write it to the dotenv artifact.
#   finish          - from the create job's `after_script`, so
#                     CI_JOB_STATUS reflects create_cluster's result.
#                     success: swap the "started" hourglass for a
#                     checkmark and strip the cluster-endpoint block the
#                     framework inserted while running. failure: swap the
#                     hourglass for a red cross, keep the endpoint block
#                     (useful for debugging a live/crashed cluster). Also
#                     sets the final e2e-framework::<success|failed>
#                     label, removing the other one first.
#   delete          - from delete/delete-auto jobs' `after_script`.
#                     Appends the cluster deletion result as a new line.
#                     Never touches the e2e-framework::* label — that
#                     reflects the create/test outcome only.
#
# finish/delete read NOTE_ID/MERGE_REQUEST_IID from the environment —
# inherited via dotenv (needs:artifacts) from e2e-ensure-build.
# COMMENT_TOKEN is inherited the same way for finish/delete; the caller
# (e2e-ensure-build, which owns FOX_TOKEN directly) must set it itself
# for `start`.
#
# Contract with the framework's own comment edits (adding the connection
# endpoint during create_cluster): wrap that content in
#   <!-- e2e-cluster-endpoint:start --> ... <!-- e2e-cluster-endpoint:end -->
# so it can be located and stripped on success.
#
# after_script runs in a separate shell from script/before_script and its
# exit code doesn't affect the job result — finish/delete are best-effort
# and never fail the job they're attached to.

MODE="${1:?usage: e2e-notify.sh <start|finish|delete> [mr_iid]}"

COMMENT_TOKEN="${COMMENT_TOKEN:-}"
if [[ -z "${COMMENT_TOKEN}" ]]; then
  echo "COMMENT_TOKEN not available; skipping e2e ${MODE} notification" >&2
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

case "${MODE}" in
  start)
    mr_iid="${2:?usage: e2e-notify.sh start <mr_iid>}"
    body="⏳ e2e test started: PROVIDER=${E2E_PROVIDER:-?} EDITION=${E2E_EDITION:-?} K8S=${KUBERNETES_VERSION:-?} — [pipeline](${CI_PIPELINE_URL:-})"
    note_id="$(api POST "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests/${mr_iid}/notes" \
      --data-urlencode "body=${body}" | jq -r '.id // empty')"
    if [[ -z "${note_id}" ]]; then
      echo "Failed to post start comment on MR ${mr_iid}" >&2
      exit 0
    fi
    echo "${note_id}"
    exit 0
    ;;
  finish|delete)
    NOTE_ID="${NOTE_ID:-}"
    MERGE_REQUEST_IID="${MERGE_REQUEST_IID:-}"
    if [[ -z "${NOTE_ID}" || -z "${MERGE_REQUEST_IID}" ]]; then
      echo "NOTE_ID/MERGE_REQUEST_IID not available; skipping e2e ${MODE} notification" >&2
      exit 0
    fi
    ;;
  *)
    echo "Unknown mode '${MODE}'; expected start|finish|delete" >&2
    exit 0
    ;;
esac

note_url="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests/${MERGE_REQUEST_IID}/notes/${NOTE_ID}"

current_body="$(api GET "${note_url}" | jq -r '.body // empty')"
if [[ -z "${current_body}" ]]; then
  echo "Could not fetch comment ${NOTE_ID} on MR ${MERGE_REQUEST_IID}; skipping update" >&2
  exit 0
fi

if [[ "${CI_JOB_STATUS:-}" == "success" ]]; then
  STATUS_LABEL="success"
else
  STATUS_LABEL="failed"
fi

case "${MODE}" in
  finish)
    if [[ "${STATUS_LABEL}" == "success" ]]; then
      # Strip the framework's cluster-endpoint block, then swap the icon.
      stripped_body="$(CURRENT_BODY="${current_body}" python3 <<'STRIP_ENDPOINT'
import os, re
body = os.environ["CURRENT_BODY"]
body = re.sub(
    r"<!-- e2e-cluster-endpoint:start -->.*?<!-- e2e-cluster-endpoint:end -->\n?",
    "",
    body,
    flags=re.DOTALL,
)
print(body, end="")
STRIP_ENDPOINT
)"
      new_body="${stripped_body//⏳/✅}"
    else
      new_body="${current_body//⏳/❌}"
    fi
    ;;
  delete)
    if [[ "${STATUS_LABEL}" == "success" ]]; then
      DELETE_LINE="🗑️ cluster deleted: success — [job](${CI_JOB_URL:-})"
    else
      DELETE_LINE="⚠️ cluster deletion failed (status=${CI_JOB_STATUS:-unknown}) — [job](${CI_JOB_URL:-})"
    fi
    new_body="$(printf '%s\n\n%s' "${current_body}" "${DELETE_LINE}")"
    ;;
esac

if api PUT "${note_url}" --data-urlencode "body=${new_body}" >/dev/null; then
  echo "Updated comment ${NOTE_ID} on MR ${MERGE_REQUEST_IID} (mode=${MODE})"
else
  echo "Failed to update comment ${NOTE_ID} on MR ${MERGE_REQUEST_IID}" >&2
fi

if [[ "${MODE}" == "finish" ]]; then
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
fi

exit 0
