{{- /*
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.
*/}}
#!/bin/bash
export LANG=C
yum updateinfo
until yum install nc curl wget jq -y; do
  echo "Error installing packages"
  yum updateinfo
  sleep 10
done
mkdir -p /var/lib/bashible/
