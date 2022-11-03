{{- /*
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.
*/}}
#!/bin/bash
export LANG=C
if ! type jq 2>/dev/null || ! type curl 2>/dev/null || ! type nc 2>/dev/null; then
  apt update
  export DEBIAN_FRONTEND=noninteractive
  until apt install jq netcat-openbsd curl -y; do
    echo "Error installing packages"
    apt update
    sleep 10
  done
fi

mkdir -p /var/lib/bashible/
