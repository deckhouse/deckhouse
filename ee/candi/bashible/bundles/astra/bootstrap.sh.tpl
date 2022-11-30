{{- /*
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.
*/}}
#!/bin/bash
export LANG=C
{{- if .proxy }}
  {{- if .proxy.httpProxy }}
HTTP_PROXY={{ .proxy.httpProxy | quote }}
  {{- end }}
  {{- if .proxy.httpsProxy }}
HTTPS_PROXY={{ .proxy.httpsProxy | quote }}
  {{- end }}
  {{- if .proxy.noProxy }}
NO_PROXY={{ .proxy.noProxy | join "," | quote }}
  {{- end }}
export HTTP_PROXY=${HTTP_PROXY}
export http_proxy=${HTTP_PROXY}
export HTTPS_PROXY=${HTTPS_PROXY}
export https_proxy=${HTTPS_PROXY}
export NO_PROXY=${NO_PROXY}
export no_proxy=${NO_PROXY}
{{- end }}
apt update
export DEBIAN_FRONTEND=noninteractive
until apt install jq netcat-openbsd curl -y; do
  echo "Error installing packages"
  apt update
  sleep 10
done
mkdir -p /var/lib/bashible/
