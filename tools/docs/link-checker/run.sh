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

# In the local env the web-frontend image carries the whole portal: the site
# pages in /app/{en,ru}, the DP documentation in /app/dkp-docs and the embedded
# module pages in /app/embedded-modules (docs/site/werf-web-frontend.inc.yaml).
export WERF_DIR=$BASEDIR/site
werf build web-frontend --env local
echo "Copying files from the web-frontend container..."
docker cp $(docker create --rm $(werf stage image web-frontend --env local)):/app ${_TMPDIR}/frontend

# Lay the files out the way .werf/nginx-local.conf serves them.
echo "Creating site structure in ${_TMPDIR}"
for lang in en ru; do
  site=${_TMPDIR}/site_${lang}
  docs=${site}/products/kubernetes-platform/documentation
  mkdir -p ${docs}
  touch ${site}/index.html
  rsync -a --exclude='en' --exclude='ru' --exclude='compare' --exclude='dkp-docs' --exclude='embedded-modules' ${_TMPDIR}/frontend/ ${site}/
  rsync -a ${_TMPDIR}/frontend/${lang}/ ${site}/
  rsync -a --exclude='en' --exclude='ru' --exclude='compare' ${_TMPDIR}/frontend/dkp-docs/ ${docs}/
  rsync -a ${_TMPDIR}/frontend/dkp-docs/${lang}/ ${docs}/
  # /$lang/gs is served under /products/kubernetes-platform/gs, the other
  # products under /products/<product>.
  mv ${site}/gs ${site}/products/kubernetes-platform/
  for product in virtualization-platform stronghold; do
    if [ -d ${site}/${product} ]; then
      mv ${site}/${product} ${site}/products/
    fi
  done
done

docker run --rm --mount type=bind,src="${_TMPDIR}/site_en",dst="/src/en",ro --mount type=bind,src="${_TMPDIR}/site_ru",dst="/src/ru",ro \
                --mount type=bind,src="./tools/docs/link-checker/entrypoint.sh",dst="/entrypoint.sh",ro ${REGISTRY_PATH}${BASE_IMAGE} /entrypoint.sh

echo "Cleaning..."
rm -rf $_TMPDIR
