{{- define "auditWebhookTemplate" }}
apiVersion: v1
kind: Config
clusters:
  - name: audit-webhook
    cluster:
      certificate-authority-data: {{ required ".webhookCA is required" .webhookCA | b64enc }}
      server: {{ required ".webhookURL" .webhookURL | quote }}
current-context: audit-webhook
contexts:
- context:
    cluster: audit-webhook
  name: audit-webhook
{{- end }}
