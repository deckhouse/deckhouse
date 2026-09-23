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
# Promote Deckhouse images for a single edition (WERF_ENV) from the stage
# registry to the prod registry (or, outside the deckhouse/deckhouse project,
# to this project's built-in GitLab Container Registry).
#
# Required env: CI_COMMIT_TAG, WERF_ENV, CI_PROJECT_PATH, CI_REGISTRY,
#   CI_REGISTRY_USER, CI_REGISTRY_PASSWORD, DECKHOUSE_REGISTRY_STAGE_HOST,
#   DECKHOUSE_REGISTRY_HOST (may be empty on forks).

set -Euo pipefail

if [[ -z "${CI_COMMIT_TAG:-}" ]]; then
  echo "CI_COMMIT_TAG is not set. Promote is allowed for release tags only." >&2
  exit 1
fi
if [[ -z "${WERF_ENV:-}" ]]; then
  echo "WERF_ENV is not set. Cannot promote unknown edition." >&2
  exit 1
fi

function regctl_copy() {
  local attempt
  for attempt in $(seq 1 5); do
    regctl image copy "$@" && return 0
    [[ ${attempt} -eq 5 ]] && return 1
    echo "⚠️ [$(date -u)] regctl copy failed (attempt ${attempt}/5), retrying in 5s..."
    sleep 5
  done
}

REGISTRY_SUFFIX="$(echo "${WERF_ENV}" | tr '[:upper:]' '[:lower:]')"
IMAGE_TAG="$(werf slugify --format docker-tag "${CI_COMMIT_TAG}")"

STAGE_PATH="${DECKHOUSE_REGISTRY_STAGE_HOST}/deckhouse/${REGISTRY_SUFFIX}"

MAIN_PROJECT=false
if [[ "${CI_PROJECT_PATH}" == "deckhouse/deckhouse" ]]; then
  MAIN_PROJECT=true
  PROD_PATH="${DECKHOUSE_REGISTRY_HOST}/deckhouse/${REGISTRY_SUFFIX}"
else
  PROD_PATH="${CI_REGISTRY}/${CI_PROJECT_PATH}/${REGISTRY_SUFFIX}"
  echo "⚓️ 🧪 [$(date -u)] CI_PROJECT_PATH='${CI_PROJECT_PATH}' is not 'deckhouse/deckhouse'. Promoting to the built-in registry: '${PROD_PATH}'"
  echo "${CI_REGISTRY_PASSWORD}" | docker login "${CI_REGISTRY}" --username "${CI_REGISTRY_USER}" --password-stdin
fi

promote_with_att() {
  local SUBPATH="$1"
  local REPO_SRC REPO_DST SRC DST
  if [[ -z "${SUBPATH}" ]]; then
    REPO_SRC="${STAGE_PATH}"
    REPO_DST="${PROD_PATH}"
  else
    REPO_SRC="${STAGE_PATH}/${SUBPATH}"
    REPO_DST="${PROD_PATH}/${SUBPATH}"
  fi
  SRC="${REPO_SRC}:${IMAGE_TAG}"
  DST="${REPO_DST}:${IMAGE_TAG}"
  echo "⚓️ 💫 [$(date -u)] Promoting '${SRC}' => '${DST}'"
  regctl_copy "${SRC}" "${DST}"
  local DIGEST
  DIGEST="$(regctl image digest "${SRC}" | sed 's/^sha256://')"
  if regctl image manifest "${REPO_SRC}:sha256-${DIGEST}.att" >/dev/null 2>&1; then
    echo "⚓️ 💫 [$(date -u)] Copying attestation for '${REPO_SRC}'"
    regctl_copy "${REPO_SRC}:sha256-${DIGEST}.att" "${REPO_DST}:sha256-${DIGEST}.att"
  else
    echo "⚓️ 💫 [$(date -u)] Attestation for '${REPO_SRC}' not found, skipping..."
  fi
}

DIGESTS_JSON="images_digests-promote.json"

if regctl image get-file "${STAGE_PATH}/install:${IMAGE_TAG}" /deckhouse/candi/images_digests.json > "${DIGESTS_JSON}" 2>/dev/null; then
  echo "⚓️ 💫 [$(date -u)] Promoting component images from images_digests.json (by digest)"
  while IFS= read -r IMG_DIGEST; do
    [[ -z "${IMG_DIGEST}" ]] && continue
    SRC_REF="${STAGE_PATH}@${IMG_DIGEST}"
    DST_REF="${PROD_PATH}@${IMG_DIGEST}"
    echo "⚓️ 💫 [$(date -u)] Promoting '${SRC_REF}' -> '${DST_REF}'"
    regctl_copy "${SRC_REF}" "${DST_REF}"
  done < <(jq -r '[.. | strings | select(test("^sha256:[a-f0-9]{64}$"))] | unique | .[]' "${DIGESTS_JSON}")
  rm -f "${DIGESTS_JSON}"
else
  echo "⚠️ [$(date -u)] Could not read /deckhouse/candi/images_digests.json from '${STAGE_PATH}/install:${IMAGE_TAG}'" >&2
  exit 1
fi

promote_with_att ""
promote_with_att "install"
promote_with_att "install-standalone"
promote_with_att "release-channel"

if [[ "${MAIN_PROJECT}" == "true" ]]; then
  EDITION="${REGISTRY_SUFFIX}"
  ./tools/check-release-images.sh --tag "${CI_COMMIT_TAG}" --edition "${EDITION}" --images-path "${DECKHOUSE_READ_REGISTRY_HOST}/deckhouse/"
else
  echo "⚓️ 🧪 [$(date -u)] Skipping public registry manifest check outside deckhouse/deckhouse."
fi
