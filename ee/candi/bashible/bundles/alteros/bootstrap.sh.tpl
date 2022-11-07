{{- /*
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.
*/}}
#!/bin/bash
export LANG=C
until yum install nc curl wget -y; do
  echo "Error installing packages"
  sleep 10
done
yum install jq -y

mkdir -p /var/lib/bashible/
