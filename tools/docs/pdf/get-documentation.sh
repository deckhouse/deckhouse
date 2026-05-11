#!/bin/bash
set -e

if [ -z "${GET_DOCUMENTATION_TMPDIR:-}" ]; then
  echo "GET_DOCUMENTATION_TMPDIR is not set. Run: make docs-generate-pdf" >&2
  exit 1
fi
if [ ! -d "${GET_DOCUMENTATION_TMPDIR}" ]; then
  echo "GET_DOCUMENTATION_TMPDIR is not a directory: ${GET_DOCUMENTATION_TMPDIR}" >&2
  exit 1
fi

_TMPDIR="${GET_DOCUMENTATION_TMPDIR}"

echo "Using temporary directory ${_TMPDIR}"
export PATH=$PATH:$PWD/bin

export WERF_DIR="docs/documentation"
export WERF_ENV="${WERF_ENV:-EE}"
export WERF_DEV=true
export WERF_BINARY=werf

if [ -n "${GITHUB_ACTIONS:-}" ]; then
  # In CI: werf ci-env sets WERF_REPO automatically from GitHub Actions environment
  type werf && source $(werf ci-env github --verbose --as-file)
else
  # Local: use localhost registry
  #export WERF_REPO="${WERF_REPO:-localhost:4999/docs}"
  export WERF_REPO="localhost:4999/docs"
  export WERF_BINARY=bin/werf
  export WERF_ENV="EE"
fi
set -x
${WERF_BINARY} build website-docs/web/static website-docs/modules-embedded/static-artifact website-docs/pdf-builder

STATIC_IMAGE=$(${WERF_BINARY} stage image website-docs/web/static 2>/dev/null | tail -1)
MODULES_IMAGE=$(${WERF_BINARY} stage image website-docs/modules-embedded/static-artifact 2>/dev/null | tail -1)
PDF_BUILDER_IMAGE=$(${WERF_BINARY} stage image website-docs/pdf-builder 2>/dev/null | tail -1)
echo "STATIC_IMAGE: ${STATIC_IMAGE}"
echo "MODULES_IMAGE: ${MODULES_IMAGE}"
echo "PDF_BUILDER_IMAGE: ${PDF_BUILDER_IMAGE}"

CONTAINER_NAME="d8-doc-${WERF_ENV,,}"
docker stop "$CONTAINER_NAME" &>/dev/null || true
docker rm "$CONTAINER_NAME" &>/dev/null || true
docker create --name "$CONTAINER_NAME" "$STATIC_IMAGE"
if [ $? -ne 0 ]; then
  echo "Error creating container!" >&2
  exit 1
fi
echo "Container was created."

mkdir -p "${_TMPDIR}/content/en" "${_TMPDIR}/content/ru" "${_TMPDIR}/embedded-modules/en" "${_TMPDIR}/embedded-modules/ru"
docker cp "$CONTAINER_NAME":/app/_site/en/. "${_TMPDIR}/content/en/"
docker cp "$CONTAINER_NAME":/app/_site/ru/. "${_TMPDIR}/content/ru/"
docker cp "$CONTAINER_NAME":/app/_site/images/. "${_TMPDIR}/content/images/"
docker cp "$CONTAINER_NAME":/app/_site/assets/. "${_TMPDIR}/content/assets/"
docker cp "$CONTAINER_NAME":/app/_site/en/. "${_TMPDIR}/embedded-modules/en/"
docker cp "$CONTAINER_NAME":/app/_site/ru/. "${_TMPDIR}/embedded-modules/ru/"
docker rm "$CONTAINER_NAME" &>/dev/null

MODULES_CONTAINER="d8-modules-${WERF_ENV,,}"
docker stop "$MODULES_CONTAINER" &>/dev/null || true
docker rm "$MODULES_CONTAINER" &>/dev/null || true
docker create --name "$MODULES_CONTAINER" "$MODULES_IMAGE"
docker cp "$MODULES_CONTAINER":/app/_site/en/modules/. "${_TMPDIR}/embedded-modules/en/modules/"
docker cp "$MODULES_CONTAINER":/app/_site/ru/modules/. "${_TMPDIR}/embedded-modules/ru/modules/"
docker rm "$MODULES_CONTAINER" &>/dev/null

# Copy static images from docs/site (not included in docs/documentation images)
cp -r "${PWD}/docs/site/images/." "${_TMPDIR}/content/images/"

echo "Data was exported to ${_TMPDIR}"
