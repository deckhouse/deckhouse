{{- define "encryptionConfigTemplate" }}
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
    - secrets
    providers:
    - aescbc:
        keys:
        - name: secretbox
          secret: {{ .secretEncryptionKey | quote }}
    - identity: {}
{{- end }}
