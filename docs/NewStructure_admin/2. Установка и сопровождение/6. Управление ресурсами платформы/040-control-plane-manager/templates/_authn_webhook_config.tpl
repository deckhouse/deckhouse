{{- define "authnWebhookTemplate" }}
apiVersion: v1
kind: Config
clusters:
  - name: user-authn-webhook
    cluster:
  {{- if .webhookCA }}
      certificate-authority-data: {{ .webhookCA | b64enc }}
  {{- end }}
      server: {{ required ".webhookURL" .webhookURL | quote }}
current-context: authn-webhook
contexts:
- context:
    cluster: user-authn-webhook
  name: authn-webhook
{{- end }}
