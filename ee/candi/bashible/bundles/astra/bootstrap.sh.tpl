{{- /*
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.
*/}}
#!/bin/bash
export LANG=C
apt update
export DEBIAN_FRONTEND=noninteractive
until apt install jq netcat-openbsd curl -y; do
  echo "Error installing packages"
  apt update
  sleep 10
done
mkdir -p /var/lib/bashible/
