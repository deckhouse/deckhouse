ca: /system_registry_pki/ca.crt
users:
  puller:
    name: {{ quote .Mirrorer.UserPuller.Name }}
    password: {{ quote .Mirrorer.UserPuller.Password }}
  pusher:
    name: {{ quote .Mirrorer.UserPusher.Name }}
    password: {{ quote .Mirrorer.UserPusher.Password }}

local: "{{ .Address }}:5001"
{{- with .Mirrorer.Upstreams }}
remote:
{{- range $ip := . }}
- "{{ $ip }}:5001"
{{- end }}
{{- else }}
remote: []
{{- end }}
