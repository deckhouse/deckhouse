# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

proxy=""
{{- if .packagesProxy }}
authstring=""
  {{- if .packagesProxy.username }}
authstring="{{ .packagesProxy.username }}"
  {{- end }}
  {{- if .packagesProxy.password }}
authstring="${authstring}:{{ .packagesProxy.password }}"
  {{- end }}
if [[ -n $authstring ]]; then
 proxy="$(echo "{{ .packagesProxy.uri }}" | sed "s/:\/\//:\/\/${authstring}@/")"
else
 proxy="{{ .packagesProxy.uri }}"
fi
{{- end }}

if [[ -n $proxy ]]; then
  bb-sync-file /etc/apt/apt.conf.d/00proxy - << EOF
Acquire {
  HTTP::proxy "$proxy";
  HTTPS::proxy "$proxy";
}
EOF
else
  rm -f /etc/apt/apt.conf.d/00proxy
fi
