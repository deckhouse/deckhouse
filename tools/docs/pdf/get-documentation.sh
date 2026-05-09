#!/bin/bash

. $(~/bin/trdl use werf 2 ea)

if [ -z "${GET_DOCUMENTATION_TMPDIR:-}" ]; then
  echo "GET_DOCUMENTATION_TMPDIR is not set. Run: make docs-generate-pdf" >&2
  exit 1
fi
if [ ! -d "${GET_DOCUMENTATION_TMPDIR}" ]; then
  echo "GET_DOCUMENTATION_TMPDIR is not a directory: ${GET_DOCUMENTATION_TMPDIR}" >&2
  exit 1
fi

_TMPDIR="${GET_DOCUMENTATION_TMPDIR}"
DOC_OUTPUT_DIR="${_TMPDIR}/content"

echo "Using temporary directory ${_TMPDIR}"
echo "Documentation output: ${DOC_OUTPUT_DIR}"
export PATH=$PATH:$PWD/bin

#source $(~/bin/trdl use werf 1.2 beta);
export CI_COMMIT_REF_NAME=dev
export CRATESPROXY=""
export CI_COMMIT_TAG=dev
export MODULE_DOC_TOKEN=d
export SOURCE_REPO=""
export GOPROXY=""
export NPMPROXY=""
export CLOUD_PROVIDERS_SOURCE_REPO=""
export OBSERVABILITY_SOURCE_REPO=""
export STRONGHOLD_PULL_TOKEN=""
export DECKHOUSE_PRIVATE_REPO=""

werf build documentation/web documentation/pdf-builder --env EE --dev --repo localhost:4999/docs
docker stop d8-doc-ee &>/dev/null
docker rm d8-doc-ee &>/dev/null

PDF_BUILDER_IMAGE=$(werf stage image documentation/pdf-builder --env EE --dev --repo localhost:4999/docs 2>/dev/null | tail -1)
echo "PDF_BUILDER_IMAGE='${PDF_BUILDER_IMAGE}'" >> "${GET_DOCUMENTATION_TMPDIR}/env"

docker create --name d8-doc-ee $(werf stage image documentation/web --env EE --dev --repo localhost:4999/docs)
if [ $? -ne 0 ]; then
  echo "Error creating container!"
  exit 1
else
  echo "Container was created."
fi

docker export -o $_TMPDIR/deckhouse-cse.tar d8-doc-ee
if [ $? -ne 0 ]; then
  echo "Error exporting data!"
  exit 1
else
  echo "Data was exported to ${_TMPDIR}"
fi

cd $_TMPDIR
if ! tar -xf deckhouse-cse.tar app/docs-dkp app/embedded-modules; then
  echo "Error extracting archive!" >&2
  exit 1
fi
rm -f "${_TMPDIR}/deckhouse-cse.tar"
