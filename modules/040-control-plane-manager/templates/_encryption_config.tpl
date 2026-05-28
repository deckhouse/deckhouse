{{- define "encryptionConfigTemplate" }}
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
{{- if .apiserver.signature }}
signature:
  privKeyPath: "/etc/kubernetes/pki/signature-private.jwk"
  pubKeyPath:  "/etc/kubernetes/pki/signature-public.jwks"
  mode: {{ .apiserver.signature | lower }}
{{- end }}
{{- if .apiserver.secretEncryptionKey }}
resources:
  - resources:
    - secrets
    providers:
    - aescbc:
        keys:
        - name: secretbox
          secret: {{ .apiserver.secretEncryptionKey | quote }}
    - identity: {}
{{- end }}
{{- end }}

{{- define "encryptionConfig" }}
{{- if or (.apiserver.secretEncryptionKey) (.apiserver.signature) }}
extra-file-secret-encryption-config.yaml: {{ include "encryptionConfigTemplate" . | b64enc }}
{{- end }}
{{- end }}

{{- define "signatureEnabledString" }}
{{- $signed := "false" }}
{{- if .apiserver.signature }}
  {{- if or (eq .apiserver.signature "Enforce") (eq .apiserver.signature "Migrate") }}
    {{- $signed = "true" }}
  {{- end }}
{{- end }}
{{- $signed -}}
{{- end }}
