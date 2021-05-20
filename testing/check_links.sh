#!/bin/bash

set -e

WERF_CHANNEL=${WERF_CHANNEL:-alpha}
BASEDIR=${BASEDIR:-$(pwd)/docs}

type multiwerf && source $(multiwerf use 1.2 ${WERF_CHANNEL} --as-file)
type werf && source $(werf ci-env gitlab --verbose --as-file)

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
   --url_ignore "/localhost/,/https\:\/\/t.me/,/gitlab.com\/profile/,/.slack.com/,/habr.com/,/flant.ru/,/candi\/bashible\/bashbooster/,/..\/..\/compare\//,/compare\/ru\//,/compare\/en\//,/\.yml$/,/\.yaml$/,/\.tmpl$/,/\.tpl$/" \
   --url-swap "\/ru\/documentation\/v1\/:/doc/ru/,\/en\/documentation\/v1\/:/doc/en/,\/docs\/documentation\/images\/:/doc/images/" ${1}

if [ "$_TMPDIR" != "" ] ; then
    echo $_TMPDIR
    rm -rf $_TMPDIR
fi
