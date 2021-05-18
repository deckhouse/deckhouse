#!/bin/bash

set -e

type multiwerf && source $(multiwerf use 1.2 ${WERF_CHANNEL} --as-file)
type werf && source $(werf ci-env gitlab --verbose --as-file)

BASEDIR=${BASEDIR:-$(pwd)}

if [ -z "$_TMPDIR"  ] ; then
    echo "_TMPDIR is not specified. You should make cleanup manually."
    _TMPDIR=$(mktemp -d -t -p ${BASEDIR})
fi

cd $BASEDIR/site
docker cp $(docker create --rm $(werf stage image web-backend)):/app/root/ ${_TMPDIR}/site/

cd $BASEDIR/documentation
docker cp $(docker create --rm $(werf stage image web)):/app/ ${_TMPDIR}/site/doc/

docker run --rm -v "${_TMPDIR}/site:/src:ro" klakegg/html-proofer:3.19.1 --allow-hash-href --check-html --empty-alt-ignore \
   --url_ignore "/localhost/,/https\:\/\/t.me/,/gitlab.com\/profile/,/.slack.com/,/habr.com/,/flant.ru/,/candi\/bashible\/bashbooster/,/..\/..\/compare\//,/\.yml$/,/\.yaml$/,/\.tmpl$/,/\.tpl$/"

if [ "$_TMPDIR" != "" ] ; then
    rm -rf $_TMPDIR
fi
