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
  {{- $crdName := .crdName }}
  {{- $prefix := "dex-authenticator" }}
  {{- $fullName := printf "%s-%s" $crdName $prefix }}
  {{- if le (len $fullName) 63 }}
    {{- $fullName }}
  {{- else }}
    {{- $hash := $crdName | sha256sum | trunc 8 }}
    {{- $maxCrdNameLen := sub 63 (add (len $prefix) (len $hash) 2) }}
    {{- $truncatedName := $crdName | trunc $maxCrdNameLen }}
    {{- printf "%s-%s-%s" $truncatedName $hash $prefix }}
  {{- end }}
{{- end }}


{{- define "dex_authenticator_secret_name" }}
  {{- $crdName := .crdName }}
  {{- $prefix := "dex-authenticator" }}
  {{- $fullName := printf "%s-%s" $prefix $crdName }}
  {{- if le (len $fullName) 63 }}
    {{- $fullName }}
  {{- else }}
    {{- $hash := $crdName | sha256sum | trunc 8 }}
    {{- $maxCrdNameLen := sub 63 (add (len $prefix) (len $hash) 2) }}
    {{- $truncatedName := $crdName | trunc $maxCrdNameLen }}
    {{- printf "%s-%s-%s" $prefix $truncatedName $hash }}
  {{- end }}
{{- end }}

