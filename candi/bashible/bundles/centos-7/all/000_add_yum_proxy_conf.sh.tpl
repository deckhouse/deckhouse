{{- if .packagesProxy.url }}
yum-config-manager --save --setopt=proxy={{ .packagesProxy.url }} main
{{- else }}
yum-config-manager --save --setopt=proxy=_none_
{{- end }}
{{- if .packagesProxy.username }}
yum-config-manager --save --setopt=proxy_username={{ .packagesProxy.username }} main
{{- else }}
yum-config-manager --save --setopt=proxy_username=
{{- end }}
{{- if .packagesProxy.password }}
yum-config-manager --save --setopt=proxy_password={{ .packagesProxy.password }} main
{{- else }}
yum-config-manager --save --setopt=proxy_password=
{{- end }}
