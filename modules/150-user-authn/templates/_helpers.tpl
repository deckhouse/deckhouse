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

{{/*
Helper function to create safe names for DexAuthenticator objects
Usage: 
  - For pattern name-dex-authenticator: {{ include "dex_authenticator_name" (list . $crd.name) }}
  - For pattern name-suffix-dex-authenticator: {{ include "dex_authenticator_name" (list . $crd.name $suffix) }}
  - For pattern dex-authenticator-name: {{ include "dex_authenticator_name_reverse" (list . $crd.name) }}
*/}}
{{- define "dex_authenticator_name" }}
  {{- $context := index . 0 }}
  {{- $name := index . 1 }}
  {{- $suffix := "" }}
  {{- if gt (len .) 2 }}
    {{- $suffix = index . 2 }}
  {{- end }}
  {{- $prefix := "dex-authenticator" }}
  {{- $fullName := printf "%s%s-%s" $name $suffix $prefix }}
  {{- if le (len $fullName) 63 }}
    {{- $fullName }}
  {{- else }}
    {{- $hash := printf "%s%s" $name $suffix | sha256sum | trunc 8 }}
    {{- $maxNameLength := sub 63 (add (len $prefix) 1 (len $hash) 1) | int }}
    {{- if lt $maxNameLength 1 }}
      {{- $maxNameLength = 1 }}
    {{- end }}
    {{- $truncatedName := $name | trunc $maxNameLength }}
    {{- printf "%s-%s-%s" $truncatedName $hash $prefix }}
  {{- end }}
{{- end }}

{{/*
Helper function to create safe names with reverse pattern (prefix-name)
Usage: {{ include "dex_authenticator_name_reverse" (list . $crd.name) }}
*/}}
{{- define "dex_authenticator_name_reverse" }}
  {{- $context := index . 0 }}
  {{- $name := index . 1 }}
  {{- $prefix := "dex-authenticator" }}
  {{- $fullName := printf "%s-%s" $prefix $name }}
  {{- if le (len $fullName) 63 }}
    {{- $fullName }}
  {{- else }}
    {{- $hash := $name | sha256sum | trunc 8 }}
    {{- $maxNameLength := sub 63 (add (len $prefix) 1 (len $hash) 1) | int }}
    {{- if lt $maxNameLength 1 }}
      {{- $maxNameLength = 1 }}
    {{- end }}
    {{- $truncatedName := $name | trunc $maxNameLength }}
    {{- printf "%s-%s-%s" $prefix $truncatedName $hash }}
  {{- end }}
{{- end }}

{{/*
Helper function to create safe names with namespace pattern (name-namespace-dex-authenticator)
Usage: {{ include "dex_authenticator_name_with_namespace" (list . $crd.name $crd.namespace) }}
*/}}
{{- define "dex_authenticator_name_with_namespace" }}
  {{- $context := index . 0 }}
  {{- $name := index . 1 }}
  {{- $namespace := index . 2 }}
  {{- $suffix := "dex-authenticator" }}
  {{- $fullName := printf "%s-%s-%s" $name $namespace $suffix }}
  {{- if le (len $fullName) 63 }}
    {{- $fullName }}
  {{- else }}
    {{- $hash := printf "%s-%s" $name $namespace | sha256sum | trunc 8 }}
    {{- $maxNameLength := sub 63 (add (len $suffix) 1 (len $hash) 1) | int }}
    {{- if lt $maxNameLength 1 }}
      {{- $maxNameLength = 1 }}
    {{- end }}
    {{- $combinedName := printf "%s-%s" $name $namespace }}
    {{- $truncatedName := $combinedName | trunc $maxNameLength }}
    {{- printf "%s-%s-%s" $truncatedName $hash $suffix }}
  {{- end }}
{{- end }}
