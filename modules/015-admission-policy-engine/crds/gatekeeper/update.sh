#!/bin/bash
echo "Update gatekeeper crds"
current_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
script_path=$(dirname "${BASH_SOURCE[0]}")
version=$(cat $script_path/../../images/gatekeeper/werf.inc.yaml | grep "gatekeeperVersion :=" | sed -n 's/.*"\(.*\)".*/\1/p')
echo Gatekeerer version: $version
git clone --depth 1 --branch  $version  https://github.com/open-policy-agent/gatekeeper.git /tmp/gatekeeper
rm $script_path/*.yaml
cp /tmp/gatekeeper/charts/gatekeeper/crds/*.yaml "${script_path}"
rm -rf /tmp/gatekeeper
