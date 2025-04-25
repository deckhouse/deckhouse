{{- define "in_cluster_proxy_version" -}}
{{- $ctx := . -}}
{{- $version := "" -}}
{{- with $ctx.Values.systemRegistry.internal.orchestrator -}}
    {{- with (((.state).in_cluster_proxy).config).version -}}
        {{- $version = . -}}
    {{- end -}}
{{- end -}}
{{- $version -}}
{{- end -}}

{{- define "in_cluster_proxy_config_files_mount_path" -}}
{{- print "/config" -}}
{{- end -}}

{{- define "in_cluster_proxy_config_files" -}}
  {{- $ctx := $.Values.systemRegistry.internal.orchestrator.state.in_cluster_proxy.config.config -}}
  {{- $files := list
    (dict "name" "auth_config.yaml"         "b64content" ((include "auth_config_file_template" $ctx) | b64enc))
    (dict "name" "distribution_config.yaml" "b64content" ((include "distribution_config_file_template" $ctx) | b64enc))
    (dict "name" "auth.crt"                 "b64content" ($ctx.auth_cert | b64enc))
    (dict "name" "auth.key"                 "b64content" ($ctx.auth_key | b64enc))
    (dict "name" "token.crt"                "b64content" ($ctx.token_cert | b64enc))
    (dict "name" "token.key"                "b64content" ($ctx.token_key | b64enc))
    (dict "name" "distribution.crt"         "b64content" ($ctx.distribution_cert | b64enc))
    (dict "name" "distribution.key"         "b64content" ($ctx.distribution_key | b64enc))
    (dict "name" "ca.crt"                   "b64content" ($ctx.ca | b64enc))
  -}}
  {{- if $ctx.upstream.ca -}}
    {{- $files = append $files (dict "name" "upstream-registry-ca.crt" "b64content" ($ctx.upstream.ca | b64enc)) -}}
  {{- end -}}
  {{- $files | toYaml -}}
{{- end -}}


{{- define "auth_config_file_template" -}}
{{- $ctx := . -}}
{{- $mount_path := (include "in_cluster_proxy_config_files_mount_path" $ctx) }}
server:
  addr: ":5051"
  real_ip_header: "X-Forwarded-For"
  certificate: "{{- $mount_path -}}/auth.crt"
  key: "{{- $mount_path -}}/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "{{- $mount_path -}}/token.crt"
  key: "{{- $mount_path -}}/token.key"
users:
  {{ $ctx.upstream.user.name | quote }}:
    password: {{ $ctx.upstream.user.password_hash | quote }}
acl:
  - match: { account: {{ $ctx.upstream.user.name | quote }} }
    actions: ["pull"]
{{- end -}}


{{- define "distribution_config_file_template" -}}
{{- $ctx := . -}}
{{- $mount_path := (include "in_cluster_proxy_config_files_mount_path" $ctx) }}
version: 0.1
log:
  level: info
storage:
  filesystem:
    rootdirectory: /data
  delete:
    enabled: true
  redirect:
    disable: true
http:
  addr: ":5001"
  prefix: /
  secret: {{ $ctx.http_secret | quote }}
  debug:
    addr: ":5002"
    prometheus:
      enabled: true
      path: /metrics
  tls:
    certificate: "{{- $mount_path -}}/distribution.crt"
    key: "{{- $mount_path -}}/distribution.key"
proxy:
  remoteurl: "{{ $ctx.upstream.scheme }}://{{ $ctx.upstream.host }}"
  username: {{ $ctx.upstream.user.name | quote }}
  password: {{ $ctx.upstream.user.password | quote }}
  remotepathonly: {{ $ctx.upstream.path | quote }}
  localpathalias: "/system/deckhouse"
  {{- with $ctx.upstream.ca }}
  ca: "{{- $mount_path -}}/upstream-registry-ca.crt"
  {{- end }}
  cache:
    disabled: true
auth:
  token:
    realm: "https://127.0.0.1:5051/auth"
    service: "Deckhouse registry"
    issuer: "Registry server"
    rootcertbundle: "{{- $mount_path -}}/token.crt"
    autoredirect: true
    proxy:
      url: "https://127.0.0.1:5051/auth"
      ca: "{{- $mount_path -}}/ca.crt"
{{- end -}}
