#!/usr/bin/env bash

# Copyright 2023 Flant JSC
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

set -Eeo pipefail

VERSION_FILE="candi/image_versions.yml"
REGISTRY_PATH="registry.deckhouse.io/base_images/"
BASE_IMAGE=$(grep -E "^BASE_JEKYLL:" ${VERSION_FILE} | head -n1 | sed 's/\"//g; s/ //g' | cut -d: -f2-)
if [ -z "$BASE_IMAGE" ]; then
  echo "Error: Base image not found in ${VERSION_FILE}"
  exit 1
fi
IMAGE_PATH="${REGISTRY_PATH}${BASE_IMAGE}"

BASEDIR=$PWD/docs
export WERF_REPO=:local
export WERF_REPO='localhost:4999/docs'
export SECONDARY_REPO=""
export DOC_API_URL=dev
export DOC_API_KEY=dev
export WERF_VERBOSE=0

_TMPDIR=$(mktemp -d -t)

# Check if the image exists in the local cache
if ! docker image inspect "${IMAGE_PATH}" > /dev/null 2>&1; then
  echo "Image ${IMAGE_PATH} not found in local cache. Pulling from registry..."
  if ! docker pull "${IMAGE_PATH}"; then
    echo "Error: Failed to pull image ${IMAGE_PATH}"
    echo "You may need to authenticate to the registry $(cut -d/ -f1 <<< ${REGISTRY_PATH}) before running the script."
    exit 1
  fi
else
  echo "Use image for $(cut -d: -f1 <<< ${BASE_IMAGE}) from the local cache (${BASE_IMAGE})."
fi

export WERF_DIR=$BASEDIR/site
werf build web-backend web-frontend --env local
echo "Copying files from the web-backend container..."
docker cp $(docker create --rm $(werf stage image web-backend --env local)):/app/root ${_TMPDIR}/backend
echo "Copying files from the web-frontend container..."
docker cp $(docker create --rm $(werf stage image web-frontend --env local)):/app ${_TMPDIR}/frontend

export WERF_DIR=$BASEDIR/documentation
werf build docs/web --env local
echo "Copying DKP documentation files from the docs/web container..."
docker cp $(docker create --rm $(werf stage image docs/web --env local)):/app ${_TMPDIR}/dkp-documentation

# Create EN site structure.
echo "Creating site structure in ${_TMPDIR}"
mkdir -p ${_TMPDIR}/site_en/products/kubernetes-platform/documentation/ ${_TMPDIR}/site_ru/products/kubernetes-platform/documentation/
touch ${_TMPDIR}/site_en/index.html ${_TMPDIR}/site_ru/index.html
rsync -a --exclude='ru' --exclude='en' --exclude='compare' --exclude='includes/header.html' ${_TMPDIR}/frontend/ ${_TMPDIR}/site_en/
rsync -a --exclude='ru' --exclude='en' --exclude='compare' --exclude='includes/header.html' ${_TMPDIR}/frontend/ ${_TMPDIR}/site_ru/
#
rsync -a ${_TMPDIR}/frontend/en/ ${_TMPDIR}/site_en/
rsync -a ${_TMPDIR}/frontend/ru/ ${_TMPDIR}/site_ru/
#
rsync -a --exclude='includes/header.html' ${_TMPDIR}/backend/en/ ${_TMPDIR}/site_en/
rsync -a --exclude='includes/header.html' ${_TMPDIR}/backend/ru/ ${_TMPDIR}/site_ru/
#
rsync -a --exclude='ru' --exclude='en' --exclude='compare' ${_TMPDIR}/dkp-documentation/ ${_TMPDIR}/site_en/products/kubernetes-platform/documentation/
rsync -a --exclude='ru' --exclude='en' --exclude='compare' ${_TMPDIR}/dkp-documentation/ ${_TMPDIR}/site_ru/products/kubernetes-platform/documentation/
rsync -a ${_TMPDIR}/dkp-documentation/en/ ${_TMPDIR}/site_en/products/kubernetes-platform/documentation/
rsync -a ${_TMPDIR}/dkp-documentation/ru/ ${_TMPDIR}/site_ru/products/kubernetes-platform/documentation/
#
rsync -a ${_TMPDIR}/dkp-documentation/{assets,images} ${_TMPDIR}/site_en/products/kubernetes-platform/documentation
rsync -a ${_TMPDIR}/dkp-documentation/{assets,images} ${_TMPDIR}/site_ru/products/kubernetes-platform/documentation

echo "Moving DKP guides and GS files..."

mv  ${_TMPDIR}/site_ru/{gs,guides} ${_TMPDIR}/site_ru/products/kubernetes-platform
mv  ${_TMPDIR}/site_en/{gs,guides} ${_TMPDIR}/site_en/products/kubernetes-platform

echo "Moving DVP files..."

mv  ${_TMPDIR}/site_ru/virtualization-platform ${_TMPDIR}/site_ru/products/virtualization-platform
mv  ${_TMPDIR}/site_en/virtualization-platform ${_TMPDIR}/site_en/products/virtualization-platform

docker run --rm --mount type=bind,src="${_TMPDIR}/site_en",dst="/src/en",ro --mount type=bind,src="${_TMPDIR}/site_ru",dst="/src/ru",ro \
                --mount type=bind,src="./tools/docs/link-checker/entrypoint.sh",dst="/entrypoint.sh",ro ${REGISTRY_PATH}${BASE_IMAGE} /entrypoint.sh

echo "Cleaning..."
rm -rf $_TMPDIR
