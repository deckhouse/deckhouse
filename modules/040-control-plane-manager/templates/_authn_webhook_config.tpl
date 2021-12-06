{{- define "authnWebhookTemplate" }}
apiVersion: v1
kind: Config
clusters:
  - name: user-authn-webhook
    cluster:
      certificate-authority-data: {{ required ".webhookCA is required" .webhookCA | b64enc }}
      server: {{ required ".webhookURL" .webhookURL | quote }}
current-context: authn-webhook
contexts:
- context:
    cluster: user-authn-webhook
  name: authn-webhook
{{- end }}
