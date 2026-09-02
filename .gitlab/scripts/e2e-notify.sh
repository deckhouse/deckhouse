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

# Posts/updates the e2e comment on the linked merge request. The comment
# has four independent sections, each updated in place without touching
# the others:
#   1) started        - fixed once, posted by `start`.
#   2) cluster endpoint - owned entirely by the e2e-framework itself; we
#                         only strip it on test success (see below).
#   3) tests status    - updated by `finish`.
#   4) deletion status - updated by `delete`.
#
# Three modes (first arg):
#
#   start <mr_iid>  - from e2e-ensure-build. Posts the initial comment
#                     (all four sections, sections 3/4 as "pending") and
#                     prints the created note id on stdout (all other
#                     output goes to stderr) so the caller can capture it
#                     and write it to the dotenv artifact.
#   finish          - from the create job's `after_script`, so
#                     CI_JOB_STATUS reflects create_cluster's result.
#                     Updates only the tests-status line: "passed" on
#                     success (also strips the cluster-endpoint block —
#                     no longer needed once confirmed working) or
#                     "failed" otherwise (endpoint block kept, useful for
#                     debugging a live/crashed cluster). Also sets the
#                     final e2e-framework::<success|failed> label,
#                     removing the other one first.
#   delete          - from delete/delete-auto jobs' `after_script`.
#                     Updates only the deletion-status line. Manages only
#                     e2e-framework::cluster-not-deleted (set on failure,
#                     cleared on success) — never touches the create/test
#                     e2e-framework::<success|failed> label.
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
# (already present, empty, in the comment `start` posts) so it can be
# located and stripped on success.
#
# after_script runs in a separate shell from script/before_script and its
# exit code doesn't affect the job result — finish/delete are best-effort
# and never fail the job they're attached to.

TESTS_STATUS_MARKER="<!-- e2e-tests-status -->"
DELETE_STATUS_MARKER="<!-- e2e-delete-status -->"

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

# Replace a single `<!-- marker -->rest of line` line with a new one,
# leaving everything else in the body untouched.
replace_status_line() {
  local body="$1"
  local marker="$2"
  local new_line="$3"
  MARKER="${marker}" NEW_LINE="${new_line}" CURRENT_BODY="${body}" python3 <<'REPLACE_LINE'
import os, re
body = os.environ["CURRENT_BODY"]
marker = os.environ["MARKER"]
new_line = os.environ["NEW_LINE"]
pattern = re.escape(marker) + r".*"
body = re.sub(pattern, lambda m: marker + new_line, body)
print(body, end="")
REPLACE_LINE
}

case "${MODE}" in
  start)
    mr_iid="${2:?usage: e2e-notify.sh start <mr_iid>}"
    body="$(cat <<BODY
⏳ e2e test started: PROVIDER=${E2E_PROVIDER:-?} EDITION=${E2E_EDITION:-?} K8S=${KUBERNETES_VERSION:-?} — [pipeline](${CI_PIPELINE_URL:-})

<!-- e2e-cluster-endpoint:start -->
<!-- e2e-cluster-endpoint:end -->

${TESTS_STATUS_MARKER}⏳ Tests: running

${DELETE_STATUS_MARKER}⏳ Cluster deletion: pending
BODY
)"
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
      # No longer needed once confirmed working.
      current_body="$(CURRENT_BODY="${current_body}" python3 <<'STRIP_ENDPOINT'
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
      new_body="$(replace_status_line "${current_body}" "${TESTS_STATUS_MARKER}" "✅ Tests: passed")"
    else
      new_body="$(replace_status_line "${current_body}" "${TESTS_STATUS_MARKER}" "❌ Tests: failed")"
    fi
    ;;
  delete)
    if [[ "${STATUS_LABEL}" == "success" ]]; then
      new_body="$(replace_status_line "${current_body}" "${DELETE_STATUS_MARKER}" "🗑️ Cluster deletion: success")"
    else
      new_body="$(replace_status_line "${current_body}" "${DELETE_STATUS_MARKER}" "⚠️ Cluster deletion: failed")"
    fi
    ;;
esac

if api PUT "${note_url}" --data-urlencode "body=${new_body}" >/dev/null; then
  echo "Updated comment ${NOTE_ID} on MR ${MERGE_REQUEST_IID} (mode=${MODE})"
else
  echo "Failed to update comment ${NOTE_ID} on MR ${MERGE_REQUEST_IID}" >&2
fi

mr_url="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests/${MERGE_REQUEST_IID}"

if [[ "${MODE}" == "finish" ]]; then
  OTHER_LABEL="failed"
  [[ "${STATUS_LABEL}" == "failed" ]] && OTHER_LABEL="success"

  if api PUT "${mr_url}" \
    --data-urlencode "remove_labels=e2e-framework::${OTHER_LABEL}" \
    --data-urlencode "add_labels=e2e-framework::${STATUS_LABEL}" >/dev/null; then
    echo "Set label e2e-framework::${STATUS_LABEL} on MR ${MERGE_REQUEST_IID}"
  else
    echo "Failed to set label on MR ${MERGE_REQUEST_IID}" >&2
  fi
fi

if [[ "${MODE}" == "delete" ]]; then
  # cluster-not-deleted only ever reflects deletion outcome, independent of
  # the create/test e2e-framework::<success|failed> label managed above.
  if [[ "${STATUS_LABEL}" == "success" ]]; then
    label_args=(--data-urlencode "remove_labels=e2e-framework::cluster-not-deleted")
    label_msg="cleared e2e-framework::cluster-not-deleted"
  else
    label_args=(--data-urlencode "add_labels=e2e-framework::cluster-not-deleted")
    label_msg="set e2e-framework::cluster-not-deleted"
  fi

  if api PUT "${mr_url}" "${label_args[@]}" >/dev/null; then
    echo "${label_msg} on MR ${MERGE_REQUEST_IID}"
  else
    echo "Failed to update cluster-not-deleted label on MR ${MERGE_REQUEST_IID}" >&2
  fi
fi

exit 0
