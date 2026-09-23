#!/bin/bash

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

timestamp_utc() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

log() {
  local level="INFO"
  if [[ $# -gt 1 ]]; then
    level="$1"
    shift
  fi

  printf '%s [%s] [k8s-autoupdate] %s\n' "$(timestamp_utc)" "${level}" "$*"
}

fail() {
  log "ERROR" "$*"
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Required command not found: $1"
}

require_var() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    fail "Required variable is not set: ${name}"
  fi
}

validate_non_empty() {
  local field_name="$1"
  local value="$2"
  [[ -n "${value}" ]] || fail "${field_name} cannot be empty"
}

validate_project_id() {
  local value="$1"
  [[ "${value}" =~ ^[0-9]+$ ]] || fail "PROJECT_ID must be numeric, got: ${value}"
}

validate_git_branch_name() {
  local value="$1"
  git check-ref-format --branch "${value}" >/dev/null 2>&1 || fail "Invalid git branch name: ${value}"
}

validate_label() {
  local value="$1"
  [[ -n "${value}" ]] || fail "Label cannot be empty"
  [[ "${value}" =~ ^[[:alnum:]_.:/-]+$ ]] || fail "Invalid label: ${value}"
}

validate_labels_csv() {
  local labels_csv="$1"
  local label
  IFS=',' read -r -a labels_array <<<"${labels_csv}"

  for label in "${labels_array[@]}"; do
    validate_label "${label}"
  done
}

validate_https_api_url() {
  local value="$1"
  local trusted_hosts_csv="$2"

  [[ "${value}" =~ ^https:// ]] || fail "GITLAB_API_V4_URL must use HTTPS: ${value}"

  local without_scheme host_port host
  without_scheme="${value#https://}"
  host_port="${without_scheme%%/*}"
  host="${host_port%%:*}"

  [[ -n "${host}" ]] || fail "Could not parse host from GITLAB_API_V4_URL: ${value}"

  local trusted_host found=0
  IFS=',' read -r -a trusted_hosts <<<"${trusted_hosts_csv}"
  for trusted_host in "${trusted_hosts[@]}"; do
    trusted_host="${trusted_host//[[:space:]]/}"
    [[ -z "${trusted_host}" ]] && continue
    if [[ "${host}" == "${trusted_host}" ]]; then
      found=1
      break
    fi
  done

  [[ "${found}" -eq 1 ]] || fail "Untrusted GITLAB_API_V4_URL host: ${host}. Trusted hosts: ${trusted_hosts_csv}"
}

select_token_header() {
  local token="$1"
  local token_type="$2"

  case "${token_type}" in
  private)
    GITLAB_TOKEN_HEADER_NAME="PRIVATE-TOKEN"
    ;;
  job)
    GITLAB_TOKEN_HEADER_NAME="JOB-TOKEN"
    ;;
  bearer)
    GITLAB_TOKEN_HEADER_NAME="Authorization"
    GITLAB_TOKEN_HEADER_VALUE="Bearer ${token}"
    return
    ;;
  auto)
    if [[ -n "${CI_JOB_TOKEN:-}" && "${token}" == "${CI_JOB_TOKEN}" ]]; then
      GITLAB_TOKEN_HEADER_NAME="JOB-TOKEN"
    else
      GITLAB_TOKEN_HEADER_NAME="PRIVATE-TOKEN"
    fi
    ;;
  *)
    fail "Unsupported GITLAB_TOKEN_TYPE: ${token_type}. Allowed: auto, private, job, bearer"
    ;;
  esac

  GITLAB_TOKEN_HEADER_VALUE="${token}"
}

preflight_git_context() {
  git rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail "Current directory is not a git repository"
  git remote get-url origin >/dev/null 2>&1 || fail "Git remote 'origin' is not configured"
  git ls-remote --exit-code --heads origin "${TARGET_BRANCH}" >/dev/null 2>&1 || fail "Target branch does not exist on origin: ${TARGET_BRANCH}"

  if ! git diff --quiet || ! git diff --cached --quiet || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
    fail "Working tree must be clean before running k8s-autoupdate"
  fi
}

remote_branch_exists() {
  local branch="$1"
  git ls-remote --exit-code --heads origin "${branch}" >/dev/null 2>&1
}

resolve_source_branch() {
  local base_branch="$1"

  if ! remote_branch_exists "${base_branch}"; then
    printf '%s' "${base_branch}"
    return
  fi

  local suffix attempt candidate
  suffix="${CI_PIPELINE_ID:-$(date -u +%Y%m%d%H%M%S)}"
  candidate="${base_branch}-${suffix}"
  attempt=0

  while remote_branch_exists "${candidate}"; do
    attempt=$((attempt + 1))
    if [[ "${attempt}" -ge 20 ]]; then
      fail "Could not find non-colliding source branch for base: ${base_branch}"
    fi
    candidate="${base_branch}-${suffix}-${attempt}"
  done

  printf '%s' "${candidate}"
}

stage_changed_paths() {
  local -a tracked_changes=()
  local -a untracked_changes=()
  local path
  local count=0

  mapfile -d '' tracked_changes < <(git diff --name-only -z)
  mapfile -d '' untracked_changes < <(git ls-files --others --exclude-standard -z)

  declare -A seen=()

  for path in "${tracked_changes[@]}" "${untracked_changes[@]}"; do
    [[ -z "${path}" ]] && continue
    if [[ -z "${seen[${path}]:-}" ]]; then
      git add -- "${path}"
      seen["${path}"]=1
      count=$((count + 1))
    fi
  done

  log "INFO" "Staged ${count} changed path(s)"
}

find_existing_mr_iid() {
  local source_branch="$1"
  local target_branch="$2"
  local existing_mr_json

  existing_mr_json="$(api_get "/projects/${PROJECT_ID}/merge_requests" \
    --data-urlencode "state=opened" \
    --data-urlencode "source_branch=${source_branch}" \
    --data-urlencode "target_branch=${target_branch}" \
    --data-urlencode "per_page=1")"

  printf '%s' "${existing_mr_json}" | jq -r '.[0].iid // ""'
}

require_cmd git
require_cmd curl
require_cmd make
require_cmd jq

# GitLab CI/CD variables (with local-friendly fallbacks)
GITLAB_TOKEN="${GITLAB_TOKEN:-${GITLAB_PRIVATE_TOKEN:-${CI_JOB_TOKEN:-}}}"
PROJECT_ID="${PROJECT_ID:-${CI_PROJECT_ID:-}}"
GITLAB_API_V4_URL="${GITLAB_API_V4_URL:-${CI_API_V4_URL:-}}"
GITLAB_TOKEN_TYPE="${GITLAB_TOKEN_TYPE:-auto}"
TRUSTED_GITLAB_HOSTS="${TRUSTED_GITLAB_HOSTS:-${CI_SERVER_HOST:-gitlab.com}}"

if [[ -z "${GITLAB_API_V4_URL}" ]]; then
  if [[ -n "${GITLAB_API_URL:-}" ]]; then
    GITLAB_API_V4_URL="${GITLAB_API_URL%/}/api/v4"
  else
    fail "Set CI_API_V4_URL, GITLAB_API_V4_URL, or GITLAB_API_URL"
  fi
fi

require_var GITLAB_TOKEN
require_var PROJECT_ID

TARGET_BRANCH="${TARGET_BRANCH:-main}"
SOURCE_BRANCH_BASE="${SOURCE_BRANCH:-chore/k8s-autoupdate}"
SOURCE_BRANCH="${SOURCE_BRANCH_BASE}"
COMMIT_MESSAGE="${COMMIT_MESSAGE:-Automated Change: Update k8s patch version}"
MR_TITLE="${MR_TITLE:-[run ci] Automated Change: Update k8s patch version}"
MR_TEMPLATE_PATH="${MR_TEMPLATE_PATH:-.github/k8s_autoupdate_pull_request_template.md}"
GIT_USER_NAME="${GIT_USER_NAME:-deckhouse-bot}"
GIT_USER_EMAIL="${GIT_USER_EMAIL:-deckhouse-bot@example.com}"

CURL_RETRY_COUNT="${CURL_RETRY_COUNT:-5}"
CURL_RETRY_DELAY="${CURL_RETRY_DELAY:-2}"
CURL_CONNECT_TIMEOUT="${CURL_CONNECT_TIMEOUT:-10}"
CURL_MAX_TIME="${CURL_MAX_TIME:-120}"

UPDATED_VERSIONS_FILE="$(mktemp -t k8s-autoupdate-updated-versions.XXXXXX)"
LEGACY_UPDATED_VERSIONS_FILE="/tmp/updated-versions"
cleanup() {
  rm -f "${UPDATED_VERSIONS_FILE}" "${LEGACY_UPDATED_VERSIONS_FILE}"
}
trap cleanup EXIT INT TERM
export UPDATED_VERSIONS_FILE

validate_project_id "${PROJECT_ID}"
validate_git_branch_name "${TARGET_BRANCH}"
validate_git_branch_name "${SOURCE_BRANCH_BASE}"
validate_non_empty "COMMIT_MESSAGE" "${COMMIT_MESSAGE}"
validate_non_empty "MR_TITLE" "${MR_TITLE}"
validate_non_empty "GIT_USER_NAME" "${GIT_USER_NAME}"
validate_non_empty "GIT_USER_EMAIL" "${GIT_USER_EMAIL}"
validate_https_api_url "${GITLAB_API_V4_URL}" "${TRUSTED_GITLAB_HOSTS}"
select_token_header "${GITLAB_TOKEN}" "${GITLAB_TOKEN_TYPE}"
preflight_git_context

CURL_COMMON_OPTS=(
  -sS
  --fail-with-body
  --retry "${CURL_RETRY_COUNT}"
  --retry-all-errors
  --retry-delay "${CURL_RETRY_DELAY}"
  --connect-timeout "${CURL_CONNECT_TIMEOUT}"
  --max-time "${CURL_MAX_TIME}"
)

api_get() {
  local endpoint="$1"
  shift

  curl "${CURL_COMMON_OPTS[@]}" \
    --header "${GITLAB_TOKEN_HEADER_NAME}: ${GITLAB_TOKEN_HEADER_VALUE}" \
    --get "${GITLAB_API_V4_URL}${endpoint}" "$@"
}

api_post_json() {
  local method="$1"
  local endpoint="$2"
  local json_payload="$3"

  curl "${CURL_COMMON_OPTS[@]}" \
    --request "${method}" \
    --header "${GITLAB_TOKEN_HEADER_NAME}: ${GITLAB_TOKEN_HEADER_VALUE}" \
    --header "Content-Type: application/json" \
    --data "${json_payload}" \
    "${GITLAB_API_V4_URL}${endpoint}"
}

log "INFO" "Setup phase: resolving target milestone"
MILESTONES_JSON="$(api_get "/projects/${PROJECT_ID}/milestones" --data-urlencode "state=active" --data-urlencode "per_page=100")"

MILESTONE_ID="$({
  printf '%s' "${MILESTONES_JSON}" | jq -r '
    [ .[]
      | select(.title | test("^v[0-9]+\\.[0-9]+\\.0$"))
      | { id, parts: (.title | ltrimstr("v") | split(".") | map(tonumber)) }
    ]
    | sort_by(.parts[0], .parts[1], .parts[2])
    | (last | .id // "")
  '
})"

if [[ -n "${MILESTONE_ID}" ]]; then
  log "INFO" "Found milestone id: ${MILESTONE_ID}"
else
  log "WARN" "No active milestone with patch version 0 found"
fi

log "INFO" "Update k8s patch version phase"
# Update k8s patch versions (kept from previous implementation).
make update-k8s-patch-versions
make generate

# Keep k8sVersions value from /tmp/updated-versions in shell variable for this job.
k8sVersions=""
if [[ -f /tmp/updated-versions ]]; then
  k8sVersions="$(cat /tmp/updated-versions)"
fi
echo "Updated versions: ${k8sVersions}"

UPDATED_VERSIONS=""
if [[ -s "${UPDATED_VERSIONS_FILE}" ]]; then
  UPDATED_VERSIONS="$(tr -d '\n' < "${UPDATED_VERSIONS_FILE}")"
elif [[ -s "${LEGACY_UPDATED_VERSIONS_FILE}" ]]; then
  log "WARN" "Using legacy updated versions file path: ${LEGACY_UPDATED_VERSIONS_FILE}"
  UPDATED_VERSIONS="$(tr -d '\n' < "${LEGACY_UPDATED_VERSIONS_FILE}")"
fi
log "INFO" "Updated versions: ${UPDATED_VERSIONS:-<none>}"

LABELS="e2e/run/yandex-cloud"
if [[ -n "${UPDATED_VERSIONS}" ]]; then
  LABELS="${LABELS},${UPDATED_VERSIONS}"
fi
validate_labels_csv "${LABELS}"

EXISTING_MR_IID="$(find_existing_mr_iid "${SOURCE_BRANCH_BASE}" "${TARGET_BRANCH}")"
if [[ -n "${EXISTING_MR_IID}" ]]; then
  log "INFO" "Existing MR found for ${SOURCE_BRANCH_BASE} (IID: ${EXISTING_MR_IID}), reusing branch"
  SOURCE_BRANCH="${SOURCE_BRANCH_BASE}"
else
  SOURCE_BRANCH="$(resolve_source_branch "${SOURCE_BRANCH_BASE}")"
  if [[ "${SOURCE_BRANCH}" != "${SOURCE_BRANCH_BASE}" ]]; then
    log "WARN" "Source branch collision detected. Using branch: ${SOURCE_BRANCH}"
  fi
fi
validate_git_branch_name "${SOURCE_BRANCH}"

log "INFO" "Preparing git commit"
git config user.name "${GIT_USER_NAME}"
git config user.email "${GIT_USER_EMAIL}"
git checkout -B "${SOURCE_BRANCH}"
stage_changed_paths

if git diff --cached --quiet; then
  log "INFO" "No changes detected after update. Exiting without MR creation."
  exit 0
fi

git commit -s -m "${COMMIT_MESSAGE}" -m "Signed-off-by: ${GIT_USER_NAME} <${GIT_USER_EMAIL}>"
PUSH_URL="https://oauth2:${GITLAB_TOKEN}@${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git"
git push --force "${PUSH_URL}" "HEAD:refs/heads/${SOURCE_BRANCH}"

MR_DESCRIPTION=""
if [[ -f "${MR_TEMPLATE_PATH}" ]]; then
  MR_DESCRIPTION="$(cat "${MR_TEMPLATE_PATH}")"
fi

log "INFO" "Create/Update Merge Request phase"
PAYLOAD_BASE='{
  "title": $title,
  "description": $description,
  "source_branch": $source_branch,
  "target_branch": $target_branch,
  "remove_source_branch": true,
  "labels": $labels
}'

if [[ -n "${MILESTONE_ID}" ]]; then
  PAYLOAD_BASE='{
    "title": $title,
    "description": $description,
    "source_branch": $source_branch,
    "target_branch": $target_branch,
    "remove_source_branch": true,
    "labels": $labels,
    "milestone_id": $milestone_id
  }'
fi

if [[ -n "${EXISTING_MR_IID}" ]]; then
  UPDATE_JSON="$(jq -n \
    --arg title "${MR_TITLE}" \
    --arg description "${MR_DESCRIPTION}" \
    --arg source_branch "${SOURCE_BRANCH}" \
    --arg target_branch "${TARGET_BRANCH}" \
    --arg labels "${LABELS}" \
    --argjson milestone_id "${MILESTONE_ID:-null}" \
    "${PAYLOAD_BASE}")"

  MR_RESULT="$(api_post_json "PUT" "/projects/${PROJECT_ID}/merge_requests/${EXISTING_MR_IID}" "${UPDATE_JSON}")"
else
  CREATE_JSON="$(jq -n \
    --arg title "${MR_TITLE}" \
    --arg description "${MR_DESCRIPTION}" \
    --arg source_branch "${SOURCE_BRANCH}" \
    --arg target_branch "${TARGET_BRANCH}" \
    --arg labels "${LABELS}" \
    --argjson milestone_id "${MILESTONE_ID:-null}" \
    "${PAYLOAD_BASE}")"

  MR_RESULT="$(api_post_json "POST" "/projects/${PROJECT_ID}/merge_requests" "${CREATE_JSON}")"
fi

MR_WEB_URL="$(printf '%s' "${MR_RESULT}" | jq -r '.web_url // empty')"
if [[ -z "${MR_WEB_URL}" ]]; then
  fail "Merge request was not created/updated successfully: ${MR_RESULT}"
  else
    bash ./.github/scripts/send-report.sh --webhook "k8s_update" "✅Kubernetes has been automatically updated✅\n[URL]($MR_WEB_URL)"
fi

log "INFO" "Merge request is ready: ${MR_WEB_URL}"
