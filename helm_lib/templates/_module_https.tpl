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

{{- /* Usage: {{ $https_values := include "helm_lib_https_values" . | fromYaml }} */ -}}
{{- define "helm_lib_https_values" -}}
  {{- $context := . -}}
  {{- $module_values := include "helm_lib_module_values" $context | fromYaml -}}
  {{- $mode := "" -}}
  {{- $certManagerClusterIssuerName := "" -}}

  {{- if hasKey $module_values "https" -}}
    {{- if hasKey $module_values.https "mode" -}}
      {{- $mode = $module_values.https.mode -}}
      {{- if eq $mode "CertManager" -}}
        {{- if not (hasKey $module_values.https "certManager") -}}
          {{- cat "<module>.https.certManager.clusterIssuerName is mandatory when <module>.https.mode is set to CertManager" | fail -}}
        {{- end -}}
        {{- if hasKey $module_values.https.certManager "clusterIssuerName" -}}
          {{- $certManagerClusterIssuerName = $module_values.https.certManager.clusterIssuerName -}}
        {{- else -}}
          {{- cat "<module>.https.certManager.clusterIssuerName is mandatory when <module>.https.mode is set to CertManager" | fail -}}
        {{- end -}}
      {{- end -}}
    {{- else -}}
      {{- cat "<module>.https.mode is mandatory when <module>.https is defined" | fail -}}
    {{- end -}}
  {{- end -}}

  {{- if empty $mode -}}
    {{- $mode = $context.Values.global.modules.https.mode -}}
    {{- if eq $mode "CertManager" -}}
      {{- $certManagerClusterIssuerName = $context.Values.global.modules.https.certManager.clusterIssuerName -}}
    {{- end -}}
  {{- end -}}

  {{- if not (has $mode (list "Disabled" "CertManager" "CustomCertificate" "OnlyInURI")) -}}
    {{- cat "Unknown https.mode:" $mode | fail -}}
  {{- end -}}

  {{- if and (eq $mode "CertManager") (not ($context.Values.global.enabledModules | has "cert-manager")) -}}
    {{- cat "https.mode has value CertManager but cert-manager module not enabled" | fail -}}
  {{- end -}}

mode: {{ $mode }}
  {{- if eq $mode "CertManager" }}
certManager:
  clusterIssuerName: {{ $certManagerClusterIssuerName }}
  {{- end -}}

{{- end -}}

{{- /* Usage: {{ if (include "helm_lib_module_https_mode" .) }} */ -}}
{{- define "helm_lib_module_https_mode" -}}
  {{- $context := . -}}
  {{- $https_values := include "helm_lib_https_values" $context | fromYaml -}}
  {{- $https_values.mode -}}
{{- end -}}

{{- /* Usage: {{ include "helm_lib_module_https_cert_manager_cluster_issuer_name" . }} */ -}}
{{- define "helm_lib_module_https_cert_manager_cluster_issuer_name" -}}
  {{- $context := . -}}
  {{- $https_values := include "helm_lib_https_values" $context | fromYaml -}}
  {{- $https_values.certManager.clusterIssuerName -}}
{{- end -}}

{{- /* Usage: {{ if (include "helm_lib_module_https_cert_manager_cluster_issuer_is_dns01_challenge_solver" .) }} */ -}}
{{- define "helm_lib_module_https_cert_manager_cluster_issuer_is_dns01_challenge_solver" -}}
  {{- $context := . -}}
  {{- if has (include "helm_lib_module_https_cert_manager_cluster_issuer_name" $context) (list "route53" "cloudflare" "digitalocean" "clouddns") }}
    "not empty string"
  {{- end -}}
{{- end -}}

{{- /* Usage: {{ include "helm_lib_module_https_cert_manager_acme_solver_challenge_settings" . | indent 4 }} */ -}}
{{- define "helm_lib_module_https_cert_manager_acme_solver_challenge_settings" -}}
  {{- $context := . -}}
  {{- if (include "helm_lib_module_https_cert_manager_cluster_issuer_is_dns01_challenge_solver" $context) }}
- dns01:
    provider: {{ include "helm_lib_module_https_cert_manager_cluster_issuer_name" $context }}
  {{- else }}
- http01:
    ingressClass: {{ include "helm_lib_module_ingress_class" $context | quote }}
  {{- end }}
{{- end -}}

{{- /* Usage: {{ if (include "helm_lib_module_https_ingress_tls_enabled" .) }} */ -}}
{{- define "helm_lib_module_https_ingress_tls_enabled" -}}
  {{- $context := . -}}

  {{- $mode := include "helm_lib_module_https_mode" $context -}}

  {{- if or (eq "CertManager" $mode) (eq "CustomCertificate" $mode) -}}
    not empty string
  {{- end -}}
{{- end -}}
