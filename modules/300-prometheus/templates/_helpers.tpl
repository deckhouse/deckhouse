{{- define "certmanager_cluster_issuer_name" }}
  {{- if hasKey .Values.prometheus.certificateForIngress "certmanagerClusterIssuerName" }}
    {{- if .Values.prometheus.certificateForIngress.certmanagerClusterIssuerName }}
      {{- .Values.prometheus.certificateForIngress.certmanagerClusterIssuerName }}
    {{- end }}
  {{- else if hasKey .Values.global.certificateForIngress "certmanagerClusterIssuerName" }}
    {{- if .Values.global.certificateForIngress.certmanagerClusterIssuerName }}
      {{- .Values.global.certificateForIngress.certmanagerClusterIssuerName }}
    {{- end }}
  {{- end }}
{{- end }}

{{- define "custom_certificate_secret_name" }}
  {{- if hasKey .Values.prometheus.certificateForIngress "customCertificateSecretName" }}
    {{- if .Values.prometheus.certificateForIngress.customCertificateSecretName }}
      {{- .Values.prometheus.certificateForIngress.customCertificateSecretName }}
    {{- end }}
  {{- else if hasKey .Values.global.certificateForIngress "customCertificateSecretName" }}
    {{- if .Values.global.certificateForIngress.customCertificateSecretName }}
      {{- .Values.global.certificateForIngress.customCertificateSecretName }}
    {{- end }}
  {{- end }}
{{- end }}
