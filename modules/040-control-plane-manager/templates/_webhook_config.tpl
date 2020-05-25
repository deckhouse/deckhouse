{{- define "webhookTemplate" }}
apiVersion: v1
kind: Config
clusters:
  - name: user-authz-webhook
    cluster:
      certificate-authority-data: {{ required ".webhookCA is required" .webhookCA | b64enc }}
      server: {{ required ".webhookURL" .webhookURL | quote }}
users:
  - name: user-authz-webhook
    user:
      client-certificate: /etc/kubernetes/pki/front-proxy-client.crt
      client-key: /etc/kubernetes/pki/front-proxy-client.key
current-context: webhook
contexts:
- context:
    cluster: user-authz-webhook
    user: user-authz-webhook
  name: webhook
{{- end }}
