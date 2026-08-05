#!/usr/bin/env bash
# Find edition build job on an existing pipeline for the current branch/tag,
# play/retry it if not successful, wait until success.
# Writes e2e-build.env with BRANCH (image tag for e2e-framework).
#
# Tag naming: .gitlab/build.yml
# Play job:   https://docs.gitlab.com/api/jobs/#run-a-job
# Retry job:  https://docs.gitlab.com/api/jobs/#retry-a-job
set -euo pipefail

DOTENV_FILE="${DOTENV_FILE:-e2e-build.env}"
SLEEP_SECONDS="${E2E_BUILD_POLL_SLEEP:-30}"
MAX_ATTEMPTS="${E2E_BUILD_POLL_ATTEMPTS:-480}"

E2E_EDITION="${E2E_EDITION:?E2E_EDITION is required}"
EDITION_LOWER="$(echo "${E2E_EDITION}" | tr '[:upper:]' '[:lower:]')"

api() {
  local method="$1"
  local url="$2"
  shift 2
  curl -sS --fail-with-body \
    --request "${method}" \
    --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
    "$@" \
    "${url}"
}

resolve_ref() {
  if [[ -n "${CI_COMMIT_TAG:-}" ]]; then
    echo "${CI_COMMIT_TAG}"
  elif [[ -n "${CI_COMMIT_BRANCH:-}" ]]; then
    echo "${CI_COMMIT_BRANCH}"
  else
    echo "${CI_COMMIT_REF_NAME}"
  fi
}

resolve_build_job_suffix() {
  if [[ -n "${CI_COMMIT_TAG:-}" ]]; then
    echo "tag"
    return
  fi
  if [[ "${CI_COMMIT_BRANCH:-}" == "main" ]]; then
    echo "main"
    return
  fi
  if [[ "${CI_COMMIT_BRANCH:-}" =~ ^release-[0-9]+\.[0-9]+$ ]]; then
    echo "release-branch"
    return
  fi
  # Feature branches use MR pipelines (build_*:mr).
  echo "mr"
}

resolve_image_tag_base() {
  if [[ -n "${CI_COMMIT_TAG:-}" ]]; then
    # Release tag publish uses werf slugify of the tag; BRANCH for e2e on tags
    # is the tag ref (stage registry path differs — e2e-framework TYPE_REGISTRY).
    echo "${CI_COMMIT_TAG}"
    return
  fi
  if [[ -n "${CI_MERGE_REQUEST_IID:-}" ]]; then
    echo "pr${CI_MERGE_REQUEST_IID}"
    return
  fi
  # Web/API on a branch: resolve open MR iid (MR builds use prN tags).
  local mrs
  mrs="$(api GET "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests?state=opened&source_branch=${CI_COMMIT_BRANCH}&per_page=1")"
  local iid
  iid="$(echo "${mrs}" | jq -r '.[0].iid // empty')"
  if [[ -n "${iid}" ]]; then
    echo "pr${iid}"
    return
  fi
  echo "${CI_COMMIT_REF_SLUG}"
}

compute_image_tag() {
  local tag_base="$1"
  local edition_lower="$2"
  if [[ -n "${CI_COMMIT_TAG:-}" ]]; then
    # Tag builds publish without edition suffix in the tag name (.gitlab/build.yml).
    echo "${tag_base}"
    return
  fi
  if [[ "${edition_lower}" == "fe" ]]; then
    echo "${tag_base}"
  else
    echo "${tag_base}-${edition_lower}"
  fi
}

build_job_name_for() {
  local edition_lower="$1"
  local suffix="$2"
  local base
  case "${edition_lower}" in
    fe) base="build_fe" ;;
    ce) base="build_ce" ;;
    ee) base="build_ee" ;;
    be) base="build_be" ;;
    se) base="build_se" ;;
    se-plus) base="build_se_plus" ;;
    cse) base="build_cse" ;;
    *)
      echo "Unsupported E2E_EDITION='${E2E_EDITION}'" >&2
      exit 1
      ;;
  esac
  echo "${base}:${suffix}"
}

list_pipelines_for_ref() {
  local ref="$1"
  api GET "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/pipelines?ref=${ref}&per_page=50"
}

list_pipeline_jobs() {
  local pipeline_id="$1"
  api GET "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/pipelines/${pipeline_id}/jobs?per_page=100"
}

# Prefer newest pipeline that contains the target build job.
find_build_job() {
  local want_name="$1"
  local ref="$2"
  local pipelines pipeline_id jobs matched
  pipelines="$(list_pipelines_for_ref "${ref}")"

  while IFS= read -r pipeline_id; do
    [[ -z "${pipeline_id}" ]] && continue
    jobs="$(list_pipeline_jobs "${pipeline_id}")"
    matched="$(echo "${jobs}" | jq -c --arg name "${want_name}" \
      '[.[] | select(.name == $name)] | first // empty')"
    if [[ -n "${matched}" && "${matched}" != "null" ]]; then
      echo "${matched}"
      return 0
    fi
  done < <(echo "${pipelines}" | jq -r 'sort_by(.id) | reverse | .[].id')

  return 1
}

play_job() {
  local job_id="$1"
  api POST "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/jobs/${job_id}/play" >/dev/null
}

retry_job() {
  local job_id="$1"
  api POST "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/jobs/${job_id}/retry"
}

get_job() {
  local job_id="$1"
  api GET "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/jobs/${job_id}"
}

wait_for_job_success() {
  local job_id="$1"
  local attempt status
  for attempt in $(seq 1 "${MAX_ATTEMPTS}"); do
    local job
    job="$(get_job "${job_id}")"
    status="$(echo "${job}" | jq -r '.status')"
    echo "Attempt ${attempt}/${MAX_ATTEMPTS}: job ${job_id} status=${status}"
    case "${status}" in
      success)
        return 0
        ;;
      failed|canceled|cancelled|skipped)
        echo "Build job ${job_id} ended with status=${status}" >&2
        return 1
        ;;
      *)
        sleep "${SLEEP_SECONDS}"
        ;;
    esac
  done
  echo "Timeout waiting for job ${job_id}" >&2
  return 1
}

write_dotenv() {
  local branch="$1"
  local job_name="$2"
  cat > "${DOTENV_FILE}" <<EOF
BRANCH=${branch}
E2E_BUILD_JOB_NAME=${job_name}
EOF
  echo "Wrote ${DOTENV_FILE}:"
  cat "${DOTENV_FILE}"
}

main() {
  local ref suffix tag_base image_tag job_name job job_id status new_job

  ref="$(resolve_ref)"
  suffix="$(resolve_build_job_suffix)"
  tag_base="$(resolve_image_tag_base)"
  image_tag="$(compute_image_tag "${tag_base}" "${EDITION_LOWER}")"
  job_name="$(build_job_name_for "${EDITION_LOWER}" "${suffix}")"

  echo "E2E_EDITION=${E2E_EDITION}"
  echo "REF=${ref}"
  echo "TARGET_BUILD_JOB=${job_name}"
  echo "IMAGE_TAG(BRANCH)=${image_tag}"

  if ! job="$(find_build_job "${job_name}" "${ref}")"; then
    echo "Build job '${job_name}' not found in pipelines for ref '${ref}'" >&2
    exit 1
  fi

  job_id="$(echo "${job}" | jq -r '.id')"
  status="$(echo "${job}" | jq -r '.status')"
  echo "Found job id=${job_id} status=${status} url=$(echo "${job}" | jq -r '.web_url // empty')"

  case "${status}" in
    success)
      echo "Build job already succeeded"
      ;;
    manual|created)
      echo "Playing build job ${job_id}"
      play_job "${job_id}"
      wait_for_job_success "${job_id}"
      ;;
    failed|canceled|cancelled|skipped)
      echo "Retrying build job ${job_id}"
      new_job="$(retry_job "${job_id}")"
      job_id="$(echo "${new_job}" | jq -r '.id')"
      echo "Retry created job id=${job_id}"
      wait_for_job_success "${job_id}"
      ;;
    running|pending|waiting_for_resource|preparing|scheduled)
      echo "Waiting for build job ${job_id}"
      wait_for_job_success "${job_id}"
      ;;
    *)
      echo "Unexpected job status '${status}' for job ${job_id}" >&2
      exit 1
      ;;
  esac

  write_dotenv "${image_tag}" "${job_name}"
}

main "$@"
