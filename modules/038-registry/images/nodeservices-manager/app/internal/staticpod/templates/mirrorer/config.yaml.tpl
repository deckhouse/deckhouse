ca: /pki/ca.crt
users:
  puller:
    name: {{ quote .UserPuller.Name }}
    password: {{ quote .UserPuller.Password }}
  pusher:
    name: {{ quote .UserPusher.Name }}
    password: {{ quote .UserPusher.Password }}

local: {{ hostPort .LocalAddress 5001 | quote }}
{{- with .Upstreams }}
remote:
{{- range $ip := . }}
- {{ hostPort $ip 5001 | quote }}
{{- end }}
{{- else }}
remote: []
{{- end }}
