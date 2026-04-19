#!/bin/bash

# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

echo "Update ratify crds"
script_path=$(dirname "${BASH_SOURCE[0]}")
version=$(cat $script_path/../../images/ratify/werf.inc.yaml | grep "ratifyVersion :=" | sed -n 's/.*"\(.*\)".*/\1/p')
echo Ratify version: $version
git clone --depth 1 --branch  $version  https://github.com/notaryproject/ratify.git /tmp/ratify
rm $script_path/*.yaml
cp /tmp/ratify/config/crd/bases/*.yaml "${script_path}"
rm -rf /tmp/ratify
