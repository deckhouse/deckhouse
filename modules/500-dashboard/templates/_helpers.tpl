{{- define "certmanager_cluster_issuer_name" }}
  {{- if hasKey .Values.dashboard.certificateForIngress "certmanagerClusterIssuerName" }}
    {{- if .Values.dashboard.certificateForIngress.certmanagerClusterIssuerName }}
      {{- .Values.dashboard.certificateForIngress.certmanagerClusterIssuerName }}
    {{- end }}
  {{- else if hasKey .Values.global.certificateForIngress "certmanagerClusterIssuerName" }}
    {{- if .Values.global.certificateForIngress.certmanagerClusterIssuerName }}
      {{- .Values.global.certificateForIngress.certmanagerClusterIssuerName }}
    {{- end }}
  {{- end }}
{{- end }}

{{- define "custom_certificate_secret_name" }}
  {{- if hasKey .Values.dashboard.certificateForIngress "customCertificateSecretName" }}
    {{- if .Values.dashboard.certificateForIngress.customCertificateSecretName }}
      {{- .Values.dashboard.certificateForIngress.customCertificateSecretName }}
    {{- end }}
  {{- else if hasKey .Values.global.certificateForIngress "customCertificateSecretName" }}
    {{- if .Values.global.certificateForIngress.customCertificateSecretName }}
      {{- .Values.global.certificateForIngress.customCertificateSecretName }}
    {{- end }}
  {{- end }}
{{- end }}

