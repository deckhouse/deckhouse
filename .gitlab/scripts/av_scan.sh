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

mkdir -p av-results

if [[ -z "${AV_NAME:-}" ]]; then
  echo "AV_NAME is empty"
  exit 1
fi

if [[ -z "${AV_SCAN_MODE:-}" ]]; then
  echo "AV_SCAN_MODE is empty"
  exit 1
fi

CONTAINER_NAME="registry_${AV_NAME}"
SCAN_IDS=""

{
  echo "CONTAINER_NAME=${CONTAINER_NAME}"
  echo "skipped=false"
} > av.env

record_scan_id() {
  local scan_id="$1"

  SCAN_IDS="${SCAN_IDS} ${scan_id}"
  SCAN_IDS="${SCAN_IDS# }"

  echo "SCAN_ID=${scan_id}" >> av.env
  echo "SCAN_IDS=${SCAN_IDS}" >> av.env
}

validate_results() {
  local scan_id="$1"
  local results="av-results/${scan_id}/scan_results_threats.md"

  if [[ ! -f "${results}" ]]; then
    echo "⚠️ The scan results file was not found"
    exit 1
  fi

  local threat_count
  local errors_count

  threat_count="$(sed -n 's/.*Images with threats | \([0-9]\+\) |.*/\1/p' "${results}" | head -1)"
  errors_count="$(sed -n 's/.*Errors | \([0-9]\+\) |.*/\1/p' "${results}" | head -1)"

  if [[ -n "${threat_count}" && "${threat_count}" -gt 0 ]]; then
    echo "❌ THREATS DETECTED: ${threat_count} images contain threats!"
    exit 1
  fi

  if [[ -n "${errors_count}" && "${errors_count}" -gt 0 ]]; then
    echo "❌ ERRORS DETECTED: ${errors_count}!"
    exit 1
  fi

  echo "✅ No threats detected (${threat_count})"
}

run_scan() {
  local ref="$1"
  local registry_host="$2"
  local registry_image_path="$3"
  local scan_id="$4"
  local only_images="${5:-}"
  local do_chown="${6:-false}"

  record_scan_id "${scan_id}"

  if [[ -n "${only_images}" ]]; then
    docker exec -e ONLY_IMAGES="${only_images}" "${CONTAINER_NAME}" \
      scanner "${ref}" "${registry_host}${registry_image_path}" "${scan_id}"
  else
    docker exec "${CONTAINER_NAME}" \
      scanner "${ref}" "${registry_host}${registry_image_path}" "${scan_id}"
  fi

  if [[ "${do_chown}" == "true" ]]; then
    local gid
    gid="$(id -g)"
    docker exec "${CONTAINER_NAME}" chown -R "${UID}:${gid}" "scans/${scan_id}"
  fi

  local workdir
  workdir="$(docker inspect -f '{{.Config.WorkingDir}}' "${CONTAINER_NAME}")"
  docker cp "${CONTAINER_NAME}:${workdir:-/}/scans/${scan_id}" av-results/

  validate_results "${scan_id}"
  echo "✅ ${CONTAINER_NAME} scan completed for ${ref}"
}

case "${AV_SCAN_MODE}" in
  mr)
    registry_image_path="/sys/deckhouse-oss"
    ref="pr${CI_MERGE_REQUEST_IID}"
    ref_sanitized="$(echo "${ref}" | sed 's/[.\//]/-/g')"
    scan_time="$(date '+%s')"
    scan_id="${scan_time}-${CI_PIPELINE_ID}-${ref_sanitized}-${CONTAINER_NAME}"
    registry_host="${DECKHOUSE_DEV_REGISTRY_HOST}"

    only_images="$(cat only_images.json 2>/dev/null || echo '[]')"
    echo "----------------------------------------------"
    if [[ -z "${only_images}" || "${only_images}" == "[]" || "${only_images}" == "null" ]]; then
      if [[ "${CHANGED_COUNT:-0}" == "0" ]]; then
        echo "Skip ${AV_NAME} scan: no rebuilt images in this MR. Nothing to scan."
        echo "skipped=true" >> av.env
        echo "----------------------------------------------"
        exit 0
      fi
      echo "Changed images detected (${CHANGED_COUNT:-0}), but no module compact keys found."
      echo "Run full ${AV_NAME} scan fallback to cover static/non-module images."
      only_images=""
    else
      keys_count="$(jq -r 'length' <<< "${only_images}")"
      echo "🎯 Delta-scan: forwarding ONLY_IMAGES (${keys_count} key(s)) to the ${AV_NAME} scanner."
      jq -r '.[] | "    • " + .' <<< "${only_images}"
    fi
    echo "----------------------------------------------"

    run_scan "${ref}" "${registry_host}" "${registry_image_path}" "${scan_id}" "${only_images}" "false"
    ;;

  tags)
    echo "SCAN_TAGS: ${SCAN_TAGS:-}"

    if [[ -z "${SCAN_TAGS:-}" ]]; then
      echo "SCAN_TAGS is empty"
      exit 1
    fi

    while IFS= read -r scan_tag_item; do
      [[ -z "${scan_tag_item}" ]] && continue

      registry_image_path="/sys/deckhouse-oss"
      edition=""
      ref="${scan_tag_item}"
      if [[ "${scan_tag_item}" == *:* ]]; then
        edition="${scan_tag_item%%:*}"
        ref="${scan_tag_item#*:}"
      fi

      ref_sanitized="$(echo "${scan_tag_item}" | sed 's/[.\/:]/-/g')"
      scan_time="$(date '+%s')"
      scan_id="${scan_time}-${CI_PIPELINE_ID}-${ref_sanitized}"

      if [[ "${ref}" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "registry read"
        if [[ -z "${edition}" ]]; then
          echo "Edition is empty for semver tag: ${scan_tag_item}"
          exit 1
        fi
        scan_id+="-${edition}"
        registry_image_path="/deckhouse/${edition}"
        registry_host="${DECKHOUSE_READ_REGISTRY_HOST}"
      elif [[ "${ref}" =~ ^release-[0-9]+\.[0-9]+$ ]]; then
        echo "registry stage"
        registry_host="${DECKHOUSE_REGISTRY_STAGE_HOST}"
      else
        echo "registry dev"
        registry_host="${DECKHOUSE_DEV_REGISTRY_HOST}"
      fi

      scan_id+="-${CONTAINER_NAME}"
      image_ref="${registry_host}${registry_image_path}:${ref}"
      

      if ! docker exec "${CONTAINER_NAME}" regctl image digest "${image_ref}" >/dev/null 2>&1; then
        echo "⚠️ Image '${image_ref}' is absent or inaccessible (regctl check failed), continuing"
        echo "SCAN_ID_skipped=${ref}" >> av.env
        continue
      fi

      run_scan "${ref}" "${registry_host}" "${registry_image_path}" "${scan_id}" "${ONLY_IMAGES:-}" "true"
    done < <(jq -r '.[]' <<< "${SCAN_TAGS}")
    ;;

  *)
    echo "Unsupported AV_SCAN_MODE: ${AV_SCAN_MODE}"
    exit 1
    ;;
esac
