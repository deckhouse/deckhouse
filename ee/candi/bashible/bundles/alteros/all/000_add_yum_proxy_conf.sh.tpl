# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

if ! rpm -q --quiet yum-utils; then
  yum install -y yum-utils
fi

proxy="_none_"
proxy_username=""
proxy_password=""

{{- if .packagesProxy.uri }}
proxy="{{ .packagesProxy.uri }}"
{{- end }}

{{- if .packagesProxy.username }}
proxy_username="{{ .packagesProxy.username }}"
{{- end }}

{{- if .packagesProxy.password }}
proxy_password="{{ .packagesProxy.password }}"
{{- end }}

yum-config-manager --save --setopt=proxy=${proxy}
yum-config-manager --save --setopt=proxy_username=${proxy_username}
yum-config-manager --save --setopt=proxy_password=${proxy_password}
