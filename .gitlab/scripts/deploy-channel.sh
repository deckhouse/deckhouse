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
# Publish a single edition (WERF_ENV) of an already-built release tag
# (CI_COMMIT_TAG) onto a release channel (RELEASE_CHANNEL) in the prod
# registry (or, outside the deckhouse/deckhouse project, in this project's
# built-in GitLab Container Registry).
#
# Required env: CI_COMMIT_TAG, WERF_ENV, RELEASE_CHANNEL, CI_PROJECT_PATH,
#   CI_REGISTRY, CI_REGISTRY_USER, CI_REGISTRY_PASSWORD,
#   DECKHOUSE_REGISTRY_HOST (may be empty on forks).

set -Euo pipefail

if [[ -z "${CI_COMMIT_TAG:-}" ]]; then
  echo "CI_COMMIT_TAG is not set. Deploy by release channel is allowed for release tags only." >&2
  exit 1
fi
if [[ -z "${WERF_ENV:-}" ]]; then
  echo "WERF_ENV is not set. Cannot deploy unknown edition." >&2
  exit 1
fi
if [[ -z "${RELEASE_CHANNEL:-}" ]]; then
  echo "RELEASE_CHANNEL is not set." >&2
  exit 1
fi

function pull_push_rmi() {
  local SRC_NAME="$1" SRC="$2" DST="$3"
  echo "⚓️ 📥 [$(date -u)] Pull '${SRC_NAME}' image as ${SRC}."
  docker pull "${SRC}"
  echo "⚓️ 🏷 [$(date -u)] Tag '${SRC_NAME}' image as ${DST}."
  docker image tag "${SRC}" "${DST}"
  echo "⚓️ 📤 [$(date -u)] Push '${SRC_NAME}' image as ${DST}."
  docker image push "${DST}"
  echo "⚓️ 🧹 [$(date -u)] Remove local tag for '${SRC_NAME}'."
  docker image rmi "${DST}" || true
}

REGISTRY_SUFFIX="$(echo "${WERF_ENV}" | tr '[:upper:]' '[:lower:]')"

if [[ "${CI_PROJECT_PATH}" == "deckhouse/deckhouse" ]]; then
  PROD_REGISTRY_PATH="${DECKHOUSE_REGISTRY_HOST}/deckhouse"
else
  PROD_REGISTRY_PATH="${CI_REGISTRY}/${CI_PROJECT_PATH}"
  echo "⚓️ 🧪 [$(date -u)] CI_PROJECT_PATH='${CI_PROJECT_PATH}' is not 'deckhouse/deckhouse'. Publishing to the built-in registry: '${PROD_REGISTRY_PATH}'"
  echo "${CI_REGISTRY_PASSWORD}" | docker login "${CI_REGISTRY}" --username "${CI_REGISTRY_USER}" --password-stdin
fi

echo "⚓️ 💫 [$(date -u)] Start publishing Deckhouse images for '${REGISTRY_SUFFIX}' edition onto '${RELEASE_CHANNEL}' release channel."

SOURCE_IMAGE="${PROD_REGISTRY_PATH}/${REGISTRY_SUFFIX}:${CI_COMMIT_TAG}"
PROD_IMAGE="${PROD_REGISTRY_PATH}/${REGISTRY_SUFFIX}:${RELEASE_CHANNEL}"

SOURCE_INSTALL_IMAGE="${PROD_REGISTRY_PATH}/${REGISTRY_SUFFIX}/install:${CI_COMMIT_TAG}"
PROD_INSTALL_IMAGE="${PROD_REGISTRY_PATH}/${REGISTRY_SUFFIX}/install:${RELEASE_CHANNEL}"

SOURCE_RELEASE_VERSION_IMAGE="${PROD_REGISTRY_PATH}/${REGISTRY_SUFFIX}/release-channel:${CI_COMMIT_TAG}"
PROD_RELEASE_VERSION_IMAGE="${PROD_REGISTRY_PATH}/${REGISTRY_SUFFIX}/release-channel:${RELEASE_CHANNEL}"

pull_push_rmi 'dev' "${SOURCE_IMAGE}" "${PROD_IMAGE}"
pull_push_rmi 'dev/install' "${SOURCE_INSTALL_IMAGE}" "${PROD_INSTALL_IMAGE}"
pull_push_rmi 'release-channel-version' "${SOURCE_RELEASE_VERSION_IMAGE}" "${PROD_RELEASE_VERSION_IMAGE}"

echo "⚓️ 🏷 [$(date -u)] Label '${PROD_RELEASE_VERSION_IMAGE}' with release date."
crane mutate -l io.deckhouse.releasedate="$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${PROD_RELEASE_VERSION_IMAGE}"

echo "⚓️ 🧹 [$(date -u)] Remove local source images."
docker image rmi "${SOURCE_IMAGE}" "${SOURCE_INSTALL_IMAGE}" "${SOURCE_RELEASE_VERSION_IMAGE}" 2>/dev/null || true

echo "Deckhouse images published for '${REGISTRY_SUFFIX}' onto '${RELEASE_CHANNEL}':"
echo "  ${SOURCE_IMAGE} -> ${PROD_IMAGE}"
echo "  ${SOURCE_INSTALL_IMAGE} -> ${PROD_INSTALL_IMAGE}"
echo "  ${SOURCE_RELEASE_VERSION_IMAGE} -> ${PROD_RELEASE_VERSION_IMAGE}"
