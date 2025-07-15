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

{{- /* Function to truncate long names with MD5 hash to ensure they don't exceed 63 characters */ -}}
{{- define "truncate_name_with_hash" -}}
  {{- $name := . -}}
  {{- $suffix := "-dex-authenticator" -}}
  {{- $fullName := printf "%s%s" $name $suffix -}}
  {{- if gt (len $fullName) 63 -}}
    {{- $hash := $name | sha256sum | trunc 8 -}}
    {{- $maxNameLength := sub 63 (add (len $suffix) (len $hash) 1) -}}
    {{- $truncatedName := $name | trunc $maxNameLength -}}
    {{- printf "%s-%s%s" $truncatedName $hash $suffix -}}
  {{- else -}}
    {{- $fullName -}}
  {{- end -}}
{{- end -}}

{{- /* Function to generate full name with namespace for initContainer */ -}}
{{- define "truncate_name_with_hash_full" -}}
  {{- $name := .name -}}
  {{- $namespace := .namespace -}}
  {{- $suffix := "-dex-authenticator" -}}
  {{- $fullName := printf "%s%s" $name $suffix -}}
  {{- if gt (len $fullName) 63 -}}
    {{- $hash := $name | sha256sum | trunc 8 -}}
    {{- $maxNameLength := sub 63 (add (len $suffix) (len $hash) 1) -}}
    {{- $truncatedName := $name | trunc $maxNameLength -}}
    {{- printf "%s-%s%s.%s" $truncatedName $hash $suffix $namespace -}}
  {{- else -}}
    {{- printf "%s%s.%s" $name $suffix $namespace -}}
  {{- end -}}
{{- end -}}
