#!/bin/bash

unset TMPDIR
_TMPDIR=$(mktemp -d -t)

if [ $? -ne 0 ]; then
  echo "Error creating temp directory!"
  exit 1
fi

echo "Created the temporary directory $_TMPDIR"

source $(~/bin/trdl use werf 1.2 beta);
export CI_COMMIT_REF_NAME=dev; export CRATESPROXY=""; export CI_COMMIT_TAG=dev; export
MODULE_DOC_TOKEN=d ; export SOURCE_REPO=; export GOPROXY=""; export NPMPROXY="";

werf build documentation/web --env EE
docker stop d8-doc-ee &>/dev/null
docker rm d8-doc-ee &>/dev/null

docker create --name d8-doc-ee $(werf stage image documentation/web --env EE)
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

cp -rf $_TMPDIR/app/platform/ru/* $_TMPDIR/documentation/
cp -rf $_TMPDIR/app/platform/*.* $_TMPDIR/documentation/
cp -rf $_TMPDIR/app/platform/images $_TMPDIR/documentation/
cp -rf $_TMPDIR/app/platform/assets $_TMPDIR/documentation/assets

echo "Result in the $_TMPDIR directory."

# rm -rf $_TMPDIR
