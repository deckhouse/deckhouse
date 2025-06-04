ca: /system_registry_pki/ca.crt
users:
  puller:
    name: {{ quote .UserPuller.Name }}
    password: {{ quote .UserPuller.Password }}
  pusher:
    name: {{ quote .UserPusher.Name }}
    password: {{ quote .UserPusher.Password }}

local: "{{ .LocalAddress }}:5001"
{{- with .Upstreams }}
remote:
{{- range $ip := . }}
- "{{ $ip }}:5001"
{{- end }}
{{- else }}
remote: []
{{- end }}
