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

REGISTRY_PATH="registry.deckhouse.io/base_images/"
IMAGE_NAME="html-proofer:5.0.9-alpine@sha256:8fceb6e8b3a6693411e7f0f651b5b9ac6e2d469c575a18c469a91be44b2926d4"
IMAGE_PATH="${REGISTRY_PATH}${IMAGE_NAME}"

BASEDIR=$PWD/docs
export WERF_REPO=:local
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
  echo "Use image for $(cut -d: -f1 <<< ${IMAGE_NAME}) from the local cache."
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
rsync -a ${_TMPDIR}/dkp-documentation/{assets,css,images,js} ${_TMPDIR}/site_en/products/kubernetes-platform/documentation
rsync -a ${_TMPDIR}/dkp-documentation/{assets,css,images,js} ${_TMPDIR}/site_ru/products/kubernetes-platform/documentation

echo "Moving DKP guides and GS files..."

mv  ${_TMPDIR}/site_ru/{gs,guides} ${_TMPDIR}/site_ru/products/kubernetes-platform
mv  ${_TMPDIR}/site_en/{gs,guides} ${_TMPDIR}/site_en/products/kubernetes-platform

echo "Moving DVP files..."

mv  ${_TMPDIR}/site_ru/virtualization-platform ${_TMPDIR}/site_ru/products/virtualization-platform
mv  ${_TMPDIR}/site_en/virtualization-platform ${_TMPDIR}/site_en/products/virtualization-platform

echo "Checking links (EN)"
docker run --rm -v "${_TMPDIR}/site_en:/src:ro" ${REGISTRY_PATH}${IMAGE_NAME} \
       --allow_missing_href --allow-hash-href --ignore-missing-alt --ignore-empty-alt \
       --ignore-urls '/^(.+deckhouse\.io)?\/privacy-policy(\/|\.html)$/,/^(.+deckhouse\.(io|ru))?\/security-policy\.html/,/^(.+deckhouse\.(io|ru))?\/products\/kubernetes-platform\/modules\/.*$/,/^(.+deckhouse\.(io|ru))?\/modules\/.*$/,/\.sslip\.io/,/^\/[^/.]+\.(svg|png|webmanifest|ico)$/,/^\/downloads\/deckhouse-cli.+\//,/\/(terms-of-service|success-stories|deckhouse-vs-kaas|services|tech-support|security|cookie-policy|community|regulations|license-rules|license|security-policy|webinars|news|education-license|partners-program|how-to-buy|moving-from-openshift|partnership-products|academy|)\/.*/,/^\/products\/(delivery-kit|stronghold|commander|observability-platform)\//,/^\/products\/enterprise_edition\.html/,/^\/products\/kubernetes-platform\/pricing\/.*/,/localhost/,/https\:\/\/t.me/,/docs-prv\.pcisecuritystandards\.org/,/gitlab.com\/profile/,/dash.cloudflare.com\/profile/,/example.com/,/vmware.com/,/.slack.com/,/habr.com/,/flant.ru/,/bcrypt-generator.com/,/candi\/bashible\/bashbooster/,/..\/..\/compare\//,/compare\/ru\//,/compare\/en\//,/\.yml$/,/\.yaml$/,/\.tmpl$/,/\.tpl$/,/^\/products\/kubernetes-platform\/$/,/^\/products\/virtualization-platform\/$/' \
       --swap-urls "https\:\/\/deckhouse\.io\/guides\/:/products/kubernetes-platform/guides/,https\:\/\/deckhouse\.io\/gs\/:/products/kubernetes-platform/gs/,https\:\/\/deckhouse\.io\/:/,\/documentation\/latest\/:/documentation/,\/documentation\/v1\/:/documentation/" \
       --ignore-files "404.html" --ignore-status-codes "0,429" .

echo "Checking links (RU)"
docker run --rm -v "${_TMPDIR}/site_ru:/src:ro" ${REGISTRY_PATH}${IMAGE_NAME} \
       --allow_missing_href --allow-hash-href --ignore-missing-alt --ignore-empty-alt \
       --ignore-urls '/^(.+deckhouse\.io)?\/privacy-policy(\/|\.html)$/,/^(.+deckhouse\.(io|ru))?\/security-policy\.html/,/^(.+deckhouse\.(io|ru))?\/products\/kubernetes-platform\/modules\/.*$/,/^(.+deckhouse\.(io|ru))?\/modules\/.*$/,/\.sslip\.io/,/^\/[^/.]+\.(svg|png|webmanifest|ico)$/,/^\/downloads\/deckhouse-cli.+\//,/\/(terms-of-service|success-stories|deckhouse-vs-kaas|services|tech-support|security|cookie-policy|community|regulations|license-rules|license|security-policy|webinars|news|education-license|partners-program|how-to-buy|moving-from-openshift|partnership-products|academy|)\/.*/,/^\/products\/(delivery-kit|stronghold|commander|observability-platform)\//,/^\/products\/enterprise_edition\.html/,/^\/products\/kubernetes-platform\/pricing\/.*/,/localhost/,/https\:\/\/t.me/,/docs-prv\.pcisecuritystandards\.org/,/gitlab.com\/profile/,/dash.cloudflare.com\/profile/,/example.com/,/vmware.com/,/.slack.com/,/habr.com/,/flant.ru/,/bcrypt-generator.com/,/candi\/bashible\/bashbooster/,/..\/..\/compare\//,/compare\/ru\//,/compare\/en\//,/\.yml$/,/\.yaml$/,/\.tmpl$/,/\.tpl$/,/^\/products\/kubernetes-platform\/$/,/^\/products\/virtualization-platform\/$/' \
       --swap-urls "https\:\/\/deckhouse\.io\/guides\/:/products/kubernetes-platform/guides/,https\:\/\/deckhouse\.io\/gs\/:/products/kubernetes-platform/gs/,https\:\/\/deckhouse\.io\/:/,\/documentation\/latest\/:/documentation/,\/documentation\/v1\/:/documentation/" \
       --ignore-files "404.html" --ignore-status-codes "0,429" .

echo "Cleaning..."
rm -rf $_TMPDIR
