{{- define "certmanager_cluster_issuer_name" }}
  {{- if hasKey .Values.userAuthn "certificateForIngress" }}
    {{- if hasKey .Values.userAuthn.certificateForIngress "certmanagerClusterIssuerName" }}
      {{- if .Values.userAuthn.certificateForIngress.certmanagerClusterIssuerName }}
        {{- .Values.userAuthn.certificateForIngress.certmanagerClusterIssuerName }}
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
  {{- if hasKey .Values.userAuthn "certificateForIngress" }}
    {{- if hasKey .Values.userAuthn.certificateForIngress "customCertificateSecretName" }}
      {{- if .Values.userAuthn.certificateForIngress.customCertificateSecretName }}
        {{- .Values.userAuthn.certificateForIngress.customCertificateSecretName }}
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

