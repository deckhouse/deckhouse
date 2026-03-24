#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TEMPLATE_DIR="${REPO_ROOT}/docs/site/backends/docs-builder-template"
OUTPUT_DIR="${OUTPUT_DIR:-${TEMPLATE_DIR}/public}"
MODULE_PATH="${MODULE_PATH:-}"
CHANNEL="${CHANNEL:-alpha}"
MODULE_VERSION="${MODULE_VERSION:-v0.1.0}"
HUGO_IMAGE="${HUGO_IMAGE:-hugomods/hugo:debian-ci-0.150.1}"
MODE="${MODE:-build}"
POLL_INTERVAL="${POLL_INTERVAL:-700ms}"

if [[ -z "${MODULE_PATH}" ]]; then
  echo "MODULE_PATH is required." >&2
  echo "Example: make docs-external-module MODULE_PATH=/path/to/module" >&2
  exit 1
fi

if [[ ! -d "${MODULE_PATH}" ]]; then
  echo "Module repository was not found: ${MODULE_PATH}" >&2
  exit 1
fi

if [[ ! -f "${MODULE_PATH}/module.yaml" ]]; then
  echo "Required file is missing: ${MODULE_PATH}/module.yaml" >&2
  exit 1
fi

if [[ ! -d "${MODULE_PATH}/docs" ]]; then
  echo "Required directory is missing: ${MODULE_PATH}/docs" >&2
  exit 1
fi

if command -v "${REPO_ROOT}/bin/yq" >/dev/null 2>&1; then
  YQ_BIN="${REPO_ROOT}/bin/yq"
elif command -v yq >/dev/null 2>&1; then
  YQ_BIN="$(command -v yq)"
else
  echo "yq is required. Run: make yq" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required and was not found in PATH." >&2
  exit 1
fi

if [[ "${MODE}" != "build" && "${MODE}" != "serve" ]]; then
  echo "Unsupported MODE: ${MODE}. Expected: build or serve." >&2
  exit 1
fi

MODULE_PATH="$(cd -- "${MODULE_PATH}" && pwd)"
MODULE_NAME="$("${YQ_BIN}" eval -r '.name' "${MODULE_PATH}/module.yaml")"

if [[ -z "${MODULE_NAME}" || "${MODULE_NAME}" == "null" ]]; then
  echo "Unable to read module name from ${MODULE_PATH}/module.yaml" >&2
  exit 1
fi

create_temp_dir() {
  local base_dir="${TMPDIR:-/tmp}"
  local temp_dir=""

  if temp_dir="$(mktemp -d "${base_dir%/}/external-module-docs.XXXXXX" 2>/dev/null)"; then
    printf '%s\n' "${temp_dir}"
    return 0
  fi

  if temp_dir="$(TMPDIR="${base_dir%/}" mktemp -d -t external-module-docs.XXXXXX 2>/dev/null)"; then
    printf '%s\n' "${temp_dir}"
    return 0
  fi

  if temp_dir="$(TMPDIR="${base_dir%/}" mktemp -d -t external-module-docs 2>/dev/null)"; then
    printf '%s\n' "${temp_dir}"
    return 0
  fi

  echo "Failed to create a temporary directory." >&2
  exit 1
}

TMP_DIR="$(create_temp_dir)"

cleanup() {
  rm -rf "${TMP_DIR}"
}

trap cleanup EXIT

mkdir -p "${TMP_DIR}/data/modules" "${OUTPUT_DIR}"

cat > "${TMP_DIR}/data/modules/channels.yaml" <<EOF
${MODULE_NAME}:
    channels:
        ${CHANNEL}:
            version: ${MODULE_VERSION}
EOF

echo "Preparing external module documentation:"
echo "  module:  ${MODULE_NAME}"
echo "  channel: ${CHANNEL}"
echo "  version: ${MODULE_VERSION}"
echo "  source:  ${MODULE_PATH}"
echo "  output:  ${OUTPUT_DIR}"
echo "  image:   ${HUGO_IMAGE}"
echo "  mode:    ${MODE}"

if [[ "${MODE}" == "serve" ]]; then
  docker_args=(
    run --rm
    --user "$(id -u):$(id -g)"
    --volume "${OUTPUT_DIR}:/out"
    --volume "${TEMPLATE_DIR}/config:/src/config:ro"
    --volume "${TEMPLATE_DIR}/i18n:/src/i18n:ro"
    --volume "${TEMPLATE_DIR}/layouts:/src/layouts:ro"
    --volume "${TEMPLATE_DIR}/data/dkp:/src/data/dkp:ro"
    --volume "${TEMPLATE_DIR}/data/channels.yaml:/src/data/channels.yaml:ro"
    --volume "${TEMPLATE_DIR}/data/helpers.yaml:/src/data/helpers.yaml:ro"
    --volume "${TEMPLATE_DIR}/data/modules_all.json:/src/data/modules_all.json:ro"
    --volume "${TMP_DIR}/data/modules/channels.yaml:/src/data/modules/channels.yaml:ro"
    --volume "${TEMPLATE_DIR}/content/modules/_index.md:/src/content/modules/_index.md:ro"
    --volume "${TEMPLATE_DIR}/content/modules/_index.ru.md:/src/content/modules/_index.ru.md:ro"
    --volume "${TEMPLATE_DIR}/content/search:/src/content/search:ro"
    --volume "${MODULE_PATH}/docs:/src/content/modules/${MODULE_NAME}/${CHANNEL}:ro"
    --volume "${MODULE_PATH}/module.yaml:/src/data/modules/${MODULE_NAME}/${CHANNEL}/module.yaml:ro"
  )

  if [[ -f "${MODULE_PATH}/oss.yaml" ]]; then
    docker_args+=(--volume "${MODULE_PATH}/oss.yaml:/src/data/modules/${MODULE_NAME}/${CHANNEL}/oss.yaml:ro")
  fi

  if [[ -f "${MODULE_PATH}/openapi/config-values.yaml" ]]; then
    docker_args+=(--volume "${MODULE_PATH}/openapi/config-values.yaml:/src/data/modules/${MODULE_NAME}/${CHANNEL}/openapi/config-values.yaml:ro")
  fi

  if [[ -f "${MODULE_PATH}/openapi/doc-ru-config-values.yaml" ]]; then
    docker_args+=(--volume "${MODULE_PATH}/openapi/doc-ru-config-values.yaml:/src/data/modules/${MODULE_NAME}/${CHANNEL}/openapi/doc-ru-config-values.yaml:ro")
  fi

  if [[ -d "${MODULE_PATH}/crds" ]]; then
    shopt -s nullglob
    root_crds=("${MODULE_PATH}"/crds/*.yaml "${MODULE_PATH}"/crds/*.yml)
    shopt -u nullglob

    for crd in "${root_crds[@]}"; do
      docker_args+=(--volume "${crd}:/src/data/modules/${MODULE_NAME}/${CHANNEL}/crds/$(basename "${crd}"):ro")
    done
  fi

  docker_args+=(
    --workdir /src
    --entrypoint hugo
    "${HUGO_IMAGE}"
    server
    --noBuildLock
    --source /src
    --destination /out
    --environment production
    --bind 0.0.0.0
    --disableFastRender
    --forceSyncStatic
    --ignoreCache
    --renderStaticToDisk
    --disableLiveReload
    --poll "${POLL_INTERVAL}"
  )

  docker "${docker_args[@]}"
else
  mkdir -p \
    "${TMP_DIR}/content/modules/${MODULE_NAME}/${CHANNEL}" \
    "${TMP_DIR}/content/search" \
    "${TMP_DIR}/data/modules/${MODULE_NAME}/${CHANNEL}"

  cp -R "${TEMPLATE_DIR}/config" "${TMP_DIR}/config"
  cp -R "${TEMPLATE_DIR}/i18n" "${TMP_DIR}/i18n"
  cp -R "${TEMPLATE_DIR}/layouts" "${TMP_DIR}/layouts"
  cp "${TEMPLATE_DIR}/data/channels.yaml" "${TMP_DIR}/data/channels.yaml"
  cp "${TEMPLATE_DIR}/data/helpers.yaml" "${TMP_DIR}/data/helpers.yaml"
  cp "${TEMPLATE_DIR}/data/modules_all.json" "${TMP_DIR}/data/modules_all.json"
  cp -R "${TEMPLATE_DIR}/data/dkp" "${TMP_DIR}/data/dkp"
  cp "${TEMPLATE_DIR}/content/modules/_index.md" "${TMP_DIR}/content/modules/_index.md"
  cp "${TEMPLATE_DIR}/content/modules/_index.ru.md" "${TMP_DIR}/content/modules/_index.ru.md"
  cp "${TEMPLATE_DIR}/content/search/search.md" "${TMP_DIR}/content/search/search.md"
  cp "${TEMPLATE_DIR}/content/search/search.ru.md" "${TMP_DIR}/content/search/search.ru.md"

  cp -R "${MODULE_PATH}/docs/." "${TMP_DIR}/content/modules/${MODULE_NAME}/${CHANNEL}/"
  cp "${MODULE_PATH}/module.yaml" "${TMP_DIR}/data/modules/${MODULE_NAME}/${CHANNEL}/module.yaml"

  if [[ -f "${MODULE_PATH}/oss.yaml" ]]; then
    cp "${MODULE_PATH}/oss.yaml" "${TMP_DIR}/data/modules/${MODULE_NAME}/${CHANNEL}/oss.yaml"
  fi

  if [[ -f "${MODULE_PATH}/openapi/config-values.yaml" || -f "${MODULE_PATH}/openapi/doc-ru-config-values.yaml" ]]; then
    mkdir -p "${TMP_DIR}/data/modules/${MODULE_NAME}/${CHANNEL}/openapi"
  fi

  if [[ -f "${MODULE_PATH}/openapi/config-values.yaml" ]]; then
    cp "${MODULE_PATH}/openapi/config-values.yaml" "${TMP_DIR}/data/modules/${MODULE_NAME}/${CHANNEL}/openapi/config-values.yaml"
  fi

  if [[ -f "${MODULE_PATH}/openapi/doc-ru-config-values.yaml" ]]; then
    cp "${MODULE_PATH}/openapi/doc-ru-config-values.yaml" "${TMP_DIR}/data/modules/${MODULE_NAME}/${CHANNEL}/openapi/doc-ru-config-values.yaml"
  fi

  if [[ -d "${MODULE_PATH}/crds" ]]; then
    shopt -s nullglob
    root_crds=("${MODULE_PATH}"/crds/*.yaml "${MODULE_PATH}"/crds/*.yml)
    shopt -u nullglob

    if (( ${#root_crds[@]} > 0 )); then
      mkdir -p "${TMP_DIR}/data/modules/${MODULE_NAME}/${CHANNEL}/crds"
      cp "${root_crds[@]}" "${TMP_DIR}/data/modules/${MODULE_NAME}/${CHANNEL}/crds/"
    fi
  fi

  docker run --rm \
    --user "$(id -u):$(id -g)" \
    --volume "${TMP_DIR}:/src" \
    --volume "${OUTPUT_DIR}:/out" \
    --workdir /src \
    --entrypoint hugo \
    "${HUGO_IMAGE}" \
    --noBuildLock \
    --source /src \
    --destination /out \
    --environment production
fi

echo "External module documentation is available in ${OUTPUT_DIR}"
