{{- /* Usage: {{ include "helm_lib_module_uri_scheme" . }} */ -}}
{{- /* return module uri scheme "http" or "https" */ -}}
{{- define "helm_lib_module_uri_scheme" -}}
  {{- $context := . -}}

  {{- if eq "Disabled" (include "helm_lib_module_https_mode" $context) -}}
    http
  {{- else -}}
    https
  {{- end -}}
{{- end -}}

{{- /* Usage: {{ if (include "helm_lib_module_https_mode" .) }} */ -}}
{{- define "helm_lib_module_https_mode" -}}
  {{- $context := . -}}

  {{- $module_values := include "helm_lib_module_values" $context | fromYaml -}}
  {{- $result := "" -}}

  {{- if hasKey $module_values "https" -}}
    {{- if hasKey $module_values.https "mode" -}}
        {{- $result = $module_values.https.mode -}}
    {{- end -}}
  {{- else if hasKey $context.Values.global.modules.https "mode" -}}
      {{- $result = $context.Values.global.modules.https.mode -}}
  {{- end -}}

  {{- if empty $result -}}
    {{- cat "modules.https.mode is not defined neither globally nor in module" | fail -}}
  {{- else if and (eq $result "CertManager") (not ($context.Values.global.enabledModules | has "cert-manager")) -}}
    {{- cat "https.mode has value CertManager but cert-manager module not enabled" | fail -}}
  {{- else -}}
    {{- $result -}}
  {{- end -}}
{{- end -}}

{{- /* Usage: {{ include "helm_lib_module_https_cert_manager_cluster_issuer_name" . }} */ -}}
{{- define "helm_lib_module_https_cert_manager_cluster_issuer_name" -}}
  {{- $context := . -}}

  {{- $module_values := include "helm_lib_module_values" $context | fromYaml -}}
  {{- $result := "" -}}

  {{- if hasKey $module_values "https" -}}
    {{- if hasKey $module_values.https "mode" -}}
      {{- if eq $module_values.https.mode "CertManager" -}}
        {{- if hasKey $module_values.https "certManager" -}}
          {{- if hasKey $module_values.https.certManager "clusterIssuerName" -}}
            {{- $result = $module_values.https.certManager.clusterIssuerName -}}
          {{- end -}}
        {{- end -}}
      {{- end -}}
    {{- end -}}
  {{- else if hasKey $context.Values.global.modules.https "mode" -}}
    {{- if eq $context.Values.global.modules.https.mode "CertManager" -}}
      {{- if hasKey $context.Values.global.modules.https "certManager" -}}
        {{- if hasKey $context.Values.global.modules.https.certManager "clusterIssuerName" -}}
          {{- $result = $context.Values.global.modules.https.certManager.clusterIssuerName -}}
        {{- end -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}

  {{- if empty $result -}}
    {{ cat "No certManager.clusterIssuerName in module or global configuration" | fail -}}
  {{- else -}}
    {{- $result -}}
  {{- end -}}
{{- end -}}

{{- /* Usage: {{ if (include "helm_lib_module_https_ingress_tls_enabled" .) }} */ -}}
{{- define "helm_lib_module_https_ingress_tls_enabled" -}}
  {{- $context := . -}}

  {{- $mode := include "helm_lib_module_https_mode" $context -}}

  {{- if or (eq "CertManager" $mode) (eq "CustomCertificate" $mode) -}}
    not empty string
  {{- end -}}
{{- end -}}
