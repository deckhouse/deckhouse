{{- define "certmanager_cluster_issuer_name" }}
  {{- if hasKey .Values.dashboard "certificateForIngress" }}
    {{- if hasKey .Values.dashboard.certificateForIngress "certmanagerClusterIssuerName" }}
      {{- if .Values.dashboard.certificateForIngress.certmanagerClusterIssuerName }}
        {{- .Values.dashboard.certificateForIngress.certmanagerClusterIssuerName }}
      {{- end }}
    {{- end }}
  {{- else if hasKey .Values.global "certificateForIngress" }}
    {{- if hasKey .Values.global.certificateForIngress "certmanagerClusterIssuerName" }}
      {{- if .Values.global.certificateForIngress.certmanagerClusterIssuerName }}
        {{- .Values.global.certificateForIngress.certmanagerClusterIssuerName }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}

{{- define "custom_certificate_secret_name" }}
  {{- if hasKey .Values.dashboard "certificateForIngress" }}
    {{- if hasKey .Values.dashboard.certificateForIngress "customCertificateSecretName" }}
      {{- if .Values.dashboard.certificateForIngress.customCertificateSecretName }}
        {{- .Values.dashboard.certificateForIngress.customCertificateSecretName }}
      {{- end }}
    {{- end }}
  {{- else if hasKey .Values.global "certificateForIngress" }}
    {{- if hasKey .Values.global.certificateForIngress "customCertificateSecretName" }}
      {{- if .Values.global.certificateForIngress.customCertificateSecretName }}
        {{- .Values.global.certificateForIngress.customCertificateSecretName }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}

