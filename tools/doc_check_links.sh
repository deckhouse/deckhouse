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

BASEDIR=$PWD/docs
export WERF_REPO=:local
export SECONDARY_REPO=""
export DOC_API_URL=dev
export DOC_API_KEY=dev
export WERF_VERBOSE=0

_TMPDIR=$(mktemp -d -t)

export WERF_DIR=$BASEDIR/site
werf build web-backend web-frontend --env local
echo "Copying files from the web-backend container..."
docker cp $(docker create --rm $(werf stage image web-backend --env local)):/app/root ${_TMPDIR}/backend
echo "Copying files from the web-frontend container..."
docker cp $(docker create --rm $(werf stage image web-frontend --env local)):/app ${_TMPDIR}/frontend

export WERF_DIR=$BASEDIR/documentation
werf build web --env local
echo "Copying files from the web container..."
docker cp $(docker create --rm $(werf stage image web --env local)):/app ${_TMPDIR}/documentation

# Create EN site structure.
echo "Creating site structure in '${_TMPDIR}/site_en/'"
mkdir -p ${_TMPDIR}/site_en/ ${_TMPDIR}/site_ru/
touch ${_TMPDIR}/site_en/index.html ${_TMPDIR}/site_ru/index.html
rsync -a --exclude='ru' --exclude='en' --exclude='compare' ${_TMPDIR}/frontend/ ${_TMPDIR}/site_en/
rsync -a --exclude='ru' --exclude='en' --exclude='compare' ${_TMPDIR}/frontend/ ${_TMPDIR}/site_ru/
#
rsync -a ${_TMPDIR}/frontend/en/ ${_TMPDIR}/site_en/
rsync -a ${_TMPDIR}/frontend/ru/ ${_TMPDIR}/site_ru/
#
rsync -a --exclude='includes/header.html' ${_TMPDIR}/backend/en/ ${_TMPDIR}/site_en/
rsync -a --exclude='includes/header.html' ${_TMPDIR}/backend/ru/ ${_TMPDIR}/site_ru/
#
rsync -a --exclude='ru' --exclude='en' --exclude='compare' ${_TMPDIR}/documentation/ ${_TMPDIR}/site_en/documentation/
rsync -a --exclude='ru' --exclude='en' --exclude='compare' ${_TMPDIR}/documentation/ ${_TMPDIR}/site_ru/documentation/
rsync -a ${_TMPDIR}/documentation/en/ ${_TMPDIR}/site_en/documentation/
rsync -a ${_TMPDIR}/documentation/ru/ ${_TMPDIR}/site_ru/documentation/
#
rsync -a ${_TMPDIR}/documentation/{assets,css,images,js} ${_TMPDIR}/site_en/documentation
rsync -a ${_TMPDIR}/documentation/{assets,css,images,js} ${_TMPDIR}/site_ru/documentation

echo "Checking links (English language)"
docker run --rm -v "${_TMPDIR}/site_en:/src:ro" klakegg/html-proofer:3.19.2 \
           --allow-hash-href --check-html --empty-alt-ignore \
           --url-ignore "/alerts.html/,/^\/(?!(gs\/|documentation\/|guides\/))/,/localhost/,/https\:\/\/t.me/,/docs-prv\.pcisecuritystandards\.org/,/gitlab.com\/profile/,/dash.cloudflare.com\/profile/,/example.com/,/vmware.com/,/.slack.com/,/habr.com/,/flant.ru/,/bcrypt-generator.com/,/candi\/bashible\/bashbooster/,/..\/..\/compare\//,/compare\/ru\//,/compare\/en\//,/\.yml$/,/\.yaml$/,/\.tmpl$/,/\.tpl$/" \
           --url-swap "https\:\/\/deckhouse.io\/:/,\/products\/kubernetes-platform\/documentation\/v1\/:/documentation/,\/products\/kubernetes-platform\/documentation\/latest\/:/documentation/,\/documentation\/v1\/:/documentation/" \
           --file_ignore "404.html,./documentation/alerts.html" \
           --http-status-ignore "0,429" ${1}

echo "Checking links (Russian language)"
docker run --rm -v "${_TMPDIR}/site_ru:/src:ro" klakegg/html-proofer:3.19.2 \
           --allow-hash-href --check-html --empty-alt-ignore \
           --url-ignore "/alerts.html/,/^\/(?!(gs\/|documentation\/|guides\/))/,/localhost/,/https\:\/\/t.me/,/docs-prv\.pcisecuritystandards\.org/,/gitlab.com\/profile/,/dash.cloudflare.com\/profile/,/example.com/,/vmware.com/,/.slack.com/,/habr.com/,/flant.ru/,/bcrypt-generator.com/,/candi\/bashible\/bashbooster/,/..\/..\/compare\//,/compare\/ru\//,/compare\/en\//,/\.yml$/,/\.yaml$/,/\.tmpl$/,/\.tpl$/" \
           --url-swap "https\:\/\/deckhouse.io\/:/,\/products\/kubernetes-platform\/documentation\/v1\/:/documentation/,\/products\/kubernetes-platform\/documentation\/latest\/:/documentation/,\/documentation\/v1\/:/documentation/" \
           --file_ignore "404.html,./documentation/alerts.html" \
           --http-status-ignore "0,429" ${1}

echo "Cleaning..."
rm -rf $_TMPDIR
