#!/bin/bash

unset TMPDIR
_TMPDIR="/tmp/cse"

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
