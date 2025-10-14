#!/bin/bash

. $(~/bin/trdl use werf 2 alpha)

#rm -rf /tmp/cse

CSE_DOC_OUTPUT_DIR=/home/kar/deckhouse/cse/modules/810-documentation/images/web/content

unset TMPDIR
_TMPDIR=$(mktemp -d -t)
#_TMPDIR=$(mktemp -d "${TMPDIR:-/tmp}/cse")

if [ $? -ne 0 ]; then
  echo "Error creating temp directory!"
  exit 1
fi


# Remove external links
# (../)+platform/modules/ -> /modules/
# (../)+platform/ -> /
# modules/[0-9]+- -> modules/
# Удалить картинки на внешние ресурсы  -  \!\[([^\[\]]+)\]\(http[^\)]+\)
# Удалить разделы про обновление DKP в FAQ и переключение между редакциями
# (https://deckhouse.(ru|io)/documentation/v1/modules/   -> /modules/
# (https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/   -> /modules/
# customAlertmanagerConfig удалить лишние ресиверы
# cert-manager - route53
# AWS, google, azure, digitalocean, hetzner, openstack
# github/gitlab
# ссылки на github.com, kubernetes.io, helm.sh и другие внешние ресурсы
# Cloud
# aquasecurity
# cisecurity
# VictoriaMetrics
# enterprise  / community
# edition
# slack
# openapispec
# типы узлов
# nodeGroup - параметры обновлений облаков, бандлов
# ingress - sourceRanges
# standby
# Deckhouse - > Deckhouse Platform Certified Security Edition
# Hugo replacements are in docs/documentation/werf-web.inc.yaml


echo "Created the temporary directory $_TMPDIR"
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

werf build documentation/web --env EE --dev --repo localhost:4999/docs
docker stop d8-doc-ee &>/dev/null
docker rm d8-doc-ee &>/dev/null

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
  echo "Data was exported."
fi

cd $_TMPDIR
tar -xf deckhouse-cse.tar app/platform

mkdir $_TMPDIR/documentation

echo "Copying files..."
rm -rf ${CSE_DOC_OUTPUT_DIR}
if [ -n "${CSE_DOC_OUTPUT_DIR}" ]; then
  rm -rf "${CSE_DOC_OUTPUT_DIR}"
  mkdir -p "${CSE_DOC_OUTPUT_DIR}" ${CSE_DOC_OUTPUT_DIR}/images ${CSE_DOC_OUTPUT_DIR}/assets ${CSE_DOC_OUTPUT_DIR}/presentation ${CSE_DOC_OUTPUT_DIR}/embedded-modules
  cp -rf $_TMPDIR/app/platform/ru/* ${CSE_DOC_OUTPUT_DIR}
  cp -rf $_TMPDIR/app/platform/*.* ${CSE_DOC_OUTPUT_DIR}
  cp -rf $_TMPDIR/app/platform/images ${CSE_DOC_OUTPUT_DIR}/
  cp -rf $_TMPDIR/app/platform/assets ${CSE_DOC_OUTPUT_DIR}/
#  cp -rf $_TMPDIR/app/platform/presentation ${CSE_DOC_OUTPUT_DIR}/
  cp -rf $_TMPDIR/app/platform/security ${CSE_DOC_OUTPUT_DIR}/
  cp -rf $_TMPDIR/app/platform/modules/ru/modules/* ${CSE_DOC_OUTPUT_DIR}/embedded-modules
  cp -rf $_TMPDIR/app/platform/modules/ru/search-embedded-modules-index.json ${CSE_DOC_OUTPUT_DIR}/embedded-modules/search-embedded-modules-index.json
  echo "Result in the ${CSE_DOC_OUTPUT_DIR} directory."
else
  echo "CSE_DOC_OUTPUT_DIR is not set!"
fi

if [ -n  $_TMPDIR ]; then
  rm -rf $_TMPDIR
fi
