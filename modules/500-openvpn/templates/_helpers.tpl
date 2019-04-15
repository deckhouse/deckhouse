{{- define "certmanager_cluster_issuer_name" }}
  {{- if hasKey .Values.openvpn.certificateForIngress "certmanagerClusterIssuerName" }}
    {{- if .Values.openvpn.certificateForIngress.certmanagerClusterIssuerName }}
      {{- .Values.openvpn.certificateForIngress.certmanagerClusterIssuerName }}
    {{- end }}
  {{- else if hasKey .Values.global.certificateForIngress "certmanagerClusterIssuerName" }}
    {{- if .Values.global.certificateForIngress.certmanagerClusterIssuerName }}
      {{- .Values.global.certificateForIngress.certmanagerClusterIssuerName }}
    {{- end }}
  {{- end }}
{{- end }}

{{- define "custom_certificate_secret_name" }}
  {{- if hasKey .Values.openvpn.certificateForIngress "customCertificateSecretName" }}
    {{- if .Values.openvpn.certificateForIngress.customCertificateSecretName }}
      {{- .Values.openvpn.certificateForIngress.customCertificateSecretName }}
    {{- end }}
  {{- else if hasKey .Values.global.certificateForIngress "customCertificateSecretName" }}
    {{- if .Values.global.certificateForIngress.customCertificateSecretName }}
      {{- .Values.global.certificateForIngress.customCertificateSecretName }}
    {{- end }}
  {{- end }}
{{- end }}

