#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

TMPDIR_BASE="${TMPDIR:-/tmp}"
WORK_DIR="$(mktemp -d "${TMPDIR_BASE}/deckhouse-get-doc.XXXXXX")"
trap 'rm -rf "${WORK_DIR}"' EXIT

echo "Using temporary directory ${WORK_DIR}"
export PATH=$PATH:${REPO_ROOT}/bin

export WERF_DIR="${REPO_ROOT}/docs/documentation"
export WERF_ENV="${WERF_ENV:-EE}"
export WERF_BINARY=werf

if [ -n "${GITHUB_ACTIONS:-}" ]; then
  type werf && source $(werf ci-env github --verbose --as-file)
else
  export WERF_REPO="localhost:4999/docs"
  export WERF_BINARY=bin/werf
  export WERF_ENV="EE"
  export WERF_DEV=true
fi

${WERF_BINARY} build website-docs/web/static website-docs/modules-embedded/static-artifact website-docs/pdf-builder

STATIC_IMAGE=$(${WERF_BINARY} stage image website-docs/web/static)
MODULES_IMAGE=$(${WERF_BINARY} stage image website-docs/modules-embedded/static-artifact)
PDF_BUILDER_IMAGE=$(${WERF_BINARY} stage image website-docs/pdf-builder)
echo "STATIC_IMAGE: ${STATIC_IMAGE}"
echo "MODULES_IMAGE: ${MODULES_IMAGE}"
echo "PDF_BUILDER_IMAGE: ${PDF_BUILDER_IMAGE}"
[[ -n "${STATIC_IMAGE}" ]] || { echo "ERROR: STATIC_IMAGE is empty" >&2; exit 1; }
[[ -n "${MODULES_IMAGE}" ]] || { echo "ERROR: MODULES_IMAGE is empty" >&2; exit 1; }
[[ -n "${PDF_BUILDER_IMAGE}" ]] || { echo "ERROR: PDF_BUILDER_IMAGE is empty" >&2; exit 1; }

CONTAINER_NAME="d8-doc-${WERF_ENV,,}"
docker stop "${CONTAINER_NAME}" &>/dev/null || true
docker rm "${CONTAINER_NAME}" &>/dev/null || true
docker create --name "${CONTAINER_NAME}" "${STATIC_IMAGE}"
echo "Container was created."

mkdir -p "${WORK_DIR}/content/en" "${WORK_DIR}/content/ru" \
         "${WORK_DIR}/embedded-modules/en" "${WORK_DIR}/embedded-modules/ru"
docker cp "${CONTAINER_NAME}":/app/_site/en/. "${WORK_DIR}/content/en/"
docker cp "${CONTAINER_NAME}":/app/_site/ru/. "${WORK_DIR}/content/ru/"
docker cp "${CONTAINER_NAME}":/app/_site/images/. "${WORK_DIR}/content/images/"
docker cp "${CONTAINER_NAME}":/app/_site/assets/. "${WORK_DIR}/content/assets/"
docker cp "${CONTAINER_NAME}":/app/_site/en/. "${WORK_DIR}/embedded-modules/en/"
docker cp "${CONTAINER_NAME}":/app/_site/ru/. "${WORK_DIR}/embedded-modules/ru/"
docker rm "${CONTAINER_NAME}" &>/dev/null

MODULES_CONTAINER="d8-modules-${WERF_ENV,,}"
docker stop "${MODULES_CONTAINER}" &>/dev/null || true
docker rm "${MODULES_CONTAINER}" &>/dev/null || true
docker create --name "${MODULES_CONTAINER}" "${MODULES_IMAGE}"
docker cp "${MODULES_CONTAINER}":/app/_site/en/modules/. "${WORK_DIR}/embedded-modules/en/modules/"
docker cp "${MODULES_CONTAINER}":/app/_site/ru/modules/. "${WORK_DIR}/embedded-modules/ru/modules/"
docker rm "${MODULES_CONTAINER}" &>/dev/null

cp -r "${REPO_ROOT}/docs/site/images/." "${WORK_DIR}/content/images/"

# Determine DOC_VERSION
if [ -n "${DOC_VERSION:-}" ]; then
  : # already set by caller
else
  BRANCH="$(cd "${REPO_ROOT}" && git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")"
  if [ "${BRANCH}" = "main" ]; then
    DOC_VERSION="latest"
  elif echo "${BRANCH}" | grep -qE 'release-[0-9]+\.[0-9]+'; then
    DOC_VERSION="$(echo "${BRANCH}" | sed 's/.*release-//' | sed 's/[^0-9.].*//')"
  else
    DOC_VERSION="dev"
  fi
fi
echo "DOC_VERSION: ${DOC_VERSION}"

PDF_OUT="${REPO_ROOT}/pdf"
mkdir -p "${PDF_OUT}"

SIDEBAR_YAML="${REPO_ROOT}/docs/documentation/_data/sidebars/main.yml"

docker run --rm \
  -w /app \
  -e PDF_OUTPUT_PATH=/out/deckhouse-admin-guide.pdf \
  -e DOC_VERSION="${DOC_VERSION}" \
  -e BUILD_LANG="${BUILD_LANG:-}" \
  -v "${WORK_DIR}/content:/app/content:ro" \
  -v "${WORK_DIR}/embedded-modules:/app/embedded-modules:ro" \
  -v "${SIDEBAR_YAML}:/app/main.yml:ro" \
  -v "${PDF_OUT}:/out" \
  "${PDF_BUILDER_IMAGE}" \
  python3 get_pdf_page.py

docker run --rm \
  -w /app \
  -e PDF_OUTPUT_PATH=/out/deckhouse-user-guide.pdf \
  -e DOC_VERSION="${DOC_VERSION}" \
  -e BUILD_LANG="${BUILD_LANG:-}" \
  -e SECTION_FILTER="Using" \
  -e GUIDE_TITLE_EN="User's guide" \
  -e GUIDE_TITLE_RU="Руководство пользователя" \
  -v "${WORK_DIR}/content:/app/content:ro" \
  -v "${WORK_DIR}/embedded-modules:/app/embedded-modules:ro" \
  -v "${SIDEBAR_YAML}:/app/main.yml:ro" \
  -v "${PDF_OUT}:/out" \
  "${PDF_BUILDER_IMAGE}" \
  python3 get_pdf_page.py
