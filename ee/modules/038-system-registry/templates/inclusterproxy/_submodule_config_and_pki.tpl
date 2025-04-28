{{/* 
  This template retrieves the version of the in-cluster proxy configuration.
  
  Example:
  {{- $ctx := .}}
  {{- if (include "in_cluster_proxy_enable" $ctx ) }}
  proxyVersion: {{ include "in_cluster_proxy_version" $ctx }}
  {{- end }}
*/}}
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

{{/* 
  This template defines the mount path for cfg files.
  
  Example:
  {{- $ctx := .}}
  volumeMounts:
    - name: cfg-volume
      mountPath: {{ include "in_cluster_proxy_cfg_files_mount_path" $ctx }}
*/}}
{{- define "in_cluster_proxy_cfg_files_mount_path" -}}
{{- print "/cfg" -}}
{{- end -}}

{{/* 
  This template defines the mount path for PKI files.
  
  Example:
  {{- $ctx := .}}
  volumeMounts:
    - name: pki-volume
      mountPath: {{ include "in_cluster_proxy_pki_files_mount_path" $ctx }}
*/}}
{{- define "in_cluster_proxy_pki_files_mount_path" -}}
{{- print "/pki" -}}
{{- end -}}

{{/* 
  This template composes the cfg files for the in-cluster proxy, encoded in base64.
  
  Example:
  {{- $ctx := .}}
  {{- if (include "in_cluster_proxy_enable" $ctx ) }}
  data:
    {{ include "in_cluster_proxy_cfg_files" $ctx | nindent 2 }}
  {{- end }}
*/}}
{{- define "in_cluster_proxy_cfg_files" -}}
  {{- $ctx := $.Values.systemRegistry.internal.orchestrator.state.in_cluster_proxy.config.config -}}
  {{- $files := dict
    "auth_cfg.yaml"         ((include "auth_cfg_file_template" $ctx) | b64enc)
    "distribution_cfg.yaml" ((include "distribution_cfg_file_template" $ctx) | b64enc)
  -}}
  {{- $files | toYaml -}}
{{- end -}}

{{/* 
  This template composes the PKI files required for secure communication, encoded in base64.
  
  Example:
  {{- $ctx := .}}
  {{- if (include "in_cluster_proxy_enable" $ctx ) }}
  data:
    {{ include "in_cluster_proxy_pki_files" $ctx | nindent 2 }}
  {{- end }}
*/}}
{{- define "in_cluster_proxy_pki_files" -}}
  {{- $ctx := $.Values.systemRegistry.internal.orchestrator.state.in_cluster_proxy.config.config -}}
  {{- $files := dict
    "auth.crt"         ($ctx.auth_cert | b64enc)
    "auth.key"         ($ctx.auth_key | b64enc)
    "token.crt"        ($ctx.token_cert | b64enc)
    "token.key"        ($ctx.token_key | b64enc)
    "distribution.crt" ($ctx.distribution_cert | b64enc)
    "distribution.key" ($ctx.distribution_key | b64enc)
    "ca.crt"           ($ctx.ca | b64enc)
  -}}
  {{- if $ctx.upstream.ca -}}
    {{- $files = merge $files (dict "upstream-registry-ca.crt" ($ctx.upstream.ca | b64enc)) -}}
  {{- end -}}
  {{- $files | toYaml -}}
{{- end -}}

{{/* 
  Template for the authentication cfg file.
  Sets up server and user credentials for token-based authentication.
  
  Example:
  {{- $ctx := .}}
  {{- if (include "in_cluster_proxy_enable" $ctx ) }}
  authCfg: |
    {{ include "auth_cfg_file_template" $ctx | nindent 2 }}
  {{- end }}
*/}}
{{- define "auth_cfg_file_template" -}}
{{- $ctx := . -}}
{{- $pki_path := include "in_cluster_proxy_pki_files_mount_path" $ctx -}}
server:
  addr: ":5051"
  real_ip_header: "X-Forwarded-For"
  certificate: "{{- $pki_path -}}/auth.crt"
  key: "{{- $pki_path -}}/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "{{- $pki_path -}}/token.crt"
  key: "{{- $pki_path -}}/token.key"
users:
  {{ $ctx.upstream.user.name | quote }}:
    password: {{ $ctx.upstream.user.password_hash | quote }}
acl:
  - match: { account: {{ $ctx.upstream.user.name | quote }} }
    actions: ["pull"]
{{- end -}}

{{/* 
  Template for the distribution cfg file.
  
  Example:
  {{- $ctx := .}}
  {{- if (include "in_cluster_proxy_enable" $ctx ) }}
  distributionCfg: |
    {{ include "distribution_cfg_file_template" $ctx | nindent 2 }}
  {{- end }}
*/}}
{{- define "distribution_cfg_file_template" -}}
{{- $ctx := . -}}
{{- $pki_path := include "in_cluster_proxy_pki_files_mount_path" $ctx -}}
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
    certificate: "{{- $pki_path -}}/distribution.crt"
    key: "{{- $pki_path -}}/distribution.key"
proxy:
  remoteurl: "{{ $ctx.upstream.scheme }}://{{ $ctx.upstream.host }}"
  username: {{ $ctx.upstream.user.name | quote }}
  password: {{ $ctx.upstream.user.password | quote }}
  remotepathonly: {{ $ctx.upstream.path | quote }}
  localpathalias: "/system/deckhouse"
  {{- with $ctx.upstream.ca }}
  ca: "{{- $pki_path -}}/upstream-registry-ca.crt"
  {{- end }}
  cache:
    disabled: true
auth:
  token:
    realm: "https://127.0.0.1:5051/auth"
    service: "Deckhouse registry"
    issuer: "Registry server"
    rootcertbundle: "{{- $pki_path -}}/token.crt"
    autoredirect: true
    proxy:
      url: "https://127.0.0.1:5051/auth"
      ca: "{{- $pki_path -}}/ca.crt"
{{- end -}}
