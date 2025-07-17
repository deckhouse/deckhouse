{{- /* We do not need to follow global logic of naming tls secrets if publish API mode is not global */ -}}
{{- define "publish_api_certificate_name" }}
  {{- if eq .Values.userAuthn.publishAPI.https.mode "Global" }}
{{- include "helm_lib_module_https_secret_name" (list . "kubernetes-tls") }}
  {{- else }}
{{- printf "kubernetes-tls-selfsigned" }}
  {{- end }}
{{- end }}


{{- define "publish_api_deploy_certificate" }}
  {{- if .Values.userAuthn.publishAPI.enabled }}
    {{- if eq .Values.userAuthn.publishAPI.https.mode "Global" -}}
      {{- if eq (include "helm_lib_module_https_mode" .) "CertManager" }}
      "not empty string"
      {{- end }}
    {{- else }}
      "not empty string"
    {{- end }}
  {{- end }}
{{- end }}


{{- define "dex_authenticator_name" }}
  {{- $name := .name }}
  {{- $suffix := .suffix | default "" }}
  {{- $prefix := "dex-authenticator" }}
  {{- $fullName := "" }}
  {{- if $suffix }}
    {{- $fullName = printf "%s-%s-%s" $name $suffix $prefix }}
  {{- else }}
    {{- $fullName = printf "%s-%s" $name $prefix }}
  {{- end }}
  {{- if le (len $fullName) 63 }}
    {{- $fullName }}
  {{- else }}
    {{- $hash := $name | sha256sum | trunc 8 }}
    {{- if $suffix }}
      {{- $maxNameLen := sub 63 (add (len $prefix) (len $hash) (len $suffix) 3) }}
      {{- if gt $maxNameLen 0 }}
        {{- $truncatedName := $name | trunc $maxNameLen }}
        {{- printf "%s-%s-%s-%s" $truncatedName $hash $suffix $prefix }}
      {{- else }}
        {{- printf "%s-%s-%s" $hash $suffix $prefix }}
      {{- end }}
    {{- else }}
      {{- $maxNameLen := sub 63 (add (len $prefix) (len $hash) 2) }}
      {{- if gt $maxNameLen 0 }}
        {{- $truncatedName := $name | trunc $maxNameLen }}
        {{- printf "%s-%s-%s" $truncatedName $hash $prefix }}
      {{- else }}
        {{- printf "%s-%s" $hash $prefix }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}


{{- define "dex_authenticator_secret_name" }}
  {{- $name := .name }}
  {{- $prefix := "dex-authenticator" }}
  {{- $fullName := printf "%s-%s" $prefix $name }}
  {{- if le (len $fullName) 63 }}
    {{- $fullName }}
  {{- else }}
    {{- $hash := $name | sha256sum | trunc 8 }}
    {{- $maxNameLen := sub 63 (add (len $prefix) (len $hash) 2) }}
    {{- if gt $maxNameLen 0 }}
      {{- $truncatedName := $name | trunc $maxNameLen }}
      {{- printf "%s-%s-%s" $prefix $truncatedName $hash }}
    {{- else }}
      {{- printf "%s-%s" $prefix $hash }}
    {{- end }}
  {{- end }}
{{- end }}

