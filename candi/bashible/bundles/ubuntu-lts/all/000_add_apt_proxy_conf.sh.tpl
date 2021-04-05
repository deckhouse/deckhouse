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
 proxy="$(echo "{{ .packagesProxy.url }}" | sed "s/:\/\//:\/\/${authstring}@/")"
else
 proxy="{{ .packagesProxy.url }}"
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
