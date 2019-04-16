{{- define "certmanager_cluster_issuer_name" }}
  {{- if hasKey .Values.openvpn "certificateForIngress" }}
    {{- if hasKey .Values.openvpn.certificateForIngress "certmanagerClusterIssuerName" }}
      {{- if .Values.openvpn.certificateForIngress.certmanagerClusterIssuerName }}
        {{- .Values.openvpn.certificateForIngress.certmanagerClusterIssuerName }}
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
  {{- if hasKey .Values.openvpn "certificateForIngress" }}
    {{- if hasKey .Values.openvpn.certificateForIngress "customCertificateSecretName" }}
      {{- if .Values.openvpn.certificateForIngress.customCertificateSecretName }}
        {{- .Values.openvpn.certificateForIngress.customCertificateSecretName }}
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

