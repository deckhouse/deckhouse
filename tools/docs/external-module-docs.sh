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

MODULE_PATH="$(cd -- "${MODULE_PATH}" && pwd)"
MODULE_NAME="$("${YQ_BIN}" eval -r '.name' "${MODULE_PATH}/module.yaml")"

if [[ -z "${MODULE_NAME}" || "${MODULE_NAME}" == "null" ]]; then
  echo "Unable to read module name from ${MODULE_PATH}/module.yaml" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/external-module-docs.XXXXXX")"

cleanup() {
  rm -rf "${TMP_DIR}"
}

trap cleanup EXIT

mkdir -p \
  "${TMP_DIR}/content/modules" \
  "${TMP_DIR}/content/search" \
  "${TMP_DIR}/content/modules/${MODULE_NAME}/${CHANNEL}" \
  "${TMP_DIR}/data/modules/${MODULE_NAME}/${CHANNEL}" \
  "${OUTPUT_DIR}"

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

cat > "${TMP_DIR}/data/modules/channels.yaml" <<EOF
${MODULE_NAME}:
    channels:
        ${CHANNEL}:
            version: ${MODULE_VERSION}
EOF

echo "Building external module documentation:"
echo "  module:  ${MODULE_NAME}"
echo "  channel: ${CHANNEL}"
echo "  version: ${MODULE_VERSION}"
echo "  source:  ${MODULE_PATH}"
echo "  output:  ${OUTPUT_DIR}"
echo "  image:   ${HUGO_IMAGE}"

docker run --rm \
  --user "$(id -u):$(id -g)" \
  --volume "${TMP_DIR}:/src" \
  --volume "${OUTPUT_DIR}:/out" \
  --workdir /src \
  --entrypoint hugo \
  "${HUGO_IMAGE}" \
  --source /src \
  --destination /out \
  --environment production

echo "External module documentation is available in ${OUTPUT_DIR}"
