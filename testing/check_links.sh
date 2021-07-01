#!/bin/bash

# Copyright 2021 Flant CJSC
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

set -e

WERF_CHANNEL=${WERF_CHANNEL:-alpha}
BASEDIR=${BASEDIR:-$(pwd)/docs}

echo ${DECKHOUSE_DEV_REGISTRY_PASSWORD} | docker login --username="${DECKHOUSE_DEV_REGISTRY_USER}" --password-stdin ${DECKHOUSE_DEV_REGISTRY_HOST} 2>/dev/null
echo ${DECKHOUSE_REGISTRY_PASSWORD} | docker login --username="${DECKHOUSE_REGISTRY_USER}" --password-stdin ${DECKHOUSE_REGISTRY_HOST} 2>/dev/null
echo ${DECKHOUSE_REGISTRY_READ_PASSWORD} | docker login --username="${DECKHOUSE_REGISTRY_READ_USER}" --password-stdin ${DECKHOUSE_REGISTRY_READ_HOST} 2>/dev/null
type multiwerf && source $(multiwerf use 1.2 ${WERF_CHANNEL} --as-file)
type werf && source $(werf ci-env gitlab --verbose --as-file)
export WERF_REPO=${DEV_REGISTRY_PATH:-"dev-registry.deckhouse.io/sys/deckhouse-oss"}


if [ -z "$_TMPDIR"  ] ; then
    echo "_TMPDIR is not specified. You should make cleanup manually."
    _TMPDIR=$(mktemp -d -t -p ${BASEDIR})
fi

cd $BASEDIR/site
docker cp $(docker create --rm $(werf stage image web-backend)):/app/root/ ${_TMPDIR}/site/

cd $BASEDIR/documentation
docker cp $(docker create --rm $(werf stage image web)):/app/ ${_TMPDIR}/site/doc/
touch ${_TMPDIR}/site/index.html
rm -Rf ${_TMPDIR}/site/doc/compare/
cp -Rf ${_TMPDIR}/site/doc/assets/ ${_TMPDIR}/site/doc/ru/
cp -Rf ${_TMPDIR}/site/doc/css/ ${_TMPDIR}/site/doc/ru/
cp -Rf ${_TMPDIR}/site/doc/images/ ${_TMPDIR}/site/doc/ru/
cp -Rf ${_TMPDIR}/site/doc/js/ ${_TMPDIR}/site/doc/ru/
cp -Rf ${_TMPDIR}/site/doc/assets/ ${_TMPDIR}/site/doc/en/
cp -Rf ${_TMPDIR}/site/doc/css/ ${_TMPDIR}/site/doc/en/
cp -Rf ${_TMPDIR}/site/doc/images/ ${_TMPDIR}/site/doc/en/
cp -Rf ${_TMPDIR}/site/doc/js/ ${_TMPDIR}/site/doc/en/

docker run --rm -v "${_TMPDIR}/site:/src:ro" klakegg/html-proofer:3.19.1 --allow-hash-href --check-html --empty-alt-ignore \
   --url_ignore "/localhost/,/https\:\/\/t.me/,/gitlab.com\/profile/,/vmware.com/,/.slack.com/,/habr.com/,/flant.ru/,/candi\/bashible\/bashbooster/,/..\/..\/compare\//,/compare\/ru\//,/compare\/en\//,/\.yml$/,/\.yaml$/,/\.tmpl$/,/\.tpl$/" \
   --url-swap "\/ru\/documentation\/$:/doc/ru/,\/ru\/documentation\/v1\/:/doc/ru/,\/en\/documentation\/$:/doc/en/,\/en\/documentation\/v1\/:/doc/en/,\/docs\/documentation\/images\/:/doc/images/" ${1}

if [ "$_TMPDIR" != "" ] ; then
    echo $_TMPDIR
    rm -rf $_TMPDIR
fi
