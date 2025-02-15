#!/bin/bash

#rm -rf /tmp/cse

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
# в AlertmanagerConfig customAlertmanagerConfig удалить лишние ресиверы
# github/gitlab
# Cloud
# enterprise
# edition
# slack
# openapispec
# типы узлов
# nodeGroup - параметры обновлений облаков, бандлов
# ingress - sourceRanges
# standby


echo "Created the temporary directory $_TMPDIR"

source $(~/bin/trdl use werf 1.2 beta);
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

werf build documentation/web --env EE --dev
docker stop d8-doc-ee &>/dev/null
docker rm d8-doc-ee &>/dev/null

docker create --name d8-doc-ee $(werf stage image documentation/web --env EE --dev)
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
rm -rf ~/Documents/flant/deckhouse/fstec/cse-d8-docs
mkdir -p ~/Documents/flant/deckhouse/fstec/cse-d8-docs
cp -rf $_TMPDIR/app/platform/ru/* ~/Documents/flant/deckhouse/fstec/cse-d8-docs
cp -rf $_TMPDIR/app/platform/*.* ~/Documents/flant/deckhouse/fstec/cse-d8-docs
cp -rf $_TMPDIR/app/platform/images ~/Documents/flant/deckhouse/fstec/cse-d8-docs
cp -rf $_TMPDIR/app/platform/assets ~/Documents/flant/deckhouse/fstec/cse-d8-docs/assets

echo "Result in the ~/Documents/flant/deckhouse/fstec/cse-d8-docs directory."

if [ -n  $_TMPDIR ]; then
  rm -rf $_TMPDIR
fi
