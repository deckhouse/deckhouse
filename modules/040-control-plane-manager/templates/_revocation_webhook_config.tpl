{{- /*
  The kubeconfig of the user-authn-revocation authorizer: the listener of the user-authz webhook,
  its CA and the client certificate of kube-apiserver, as in _webhook_config.tpl, with the path of
  the revocation check.
*/}}
{{- define "revocationWebhookTemplate" }}
apiVersion: v1
kind: Config
clusters:
  - name: user-authn-revocation
    cluster:
      certificate-authority-data: {{ required ".webhookCA is required" .webhookCA | b64enc }}
      server: {{ required ".webhookURL" .webhookURL | quote }}
users:
  - name: user-authn-revocation
    user:
      client-certificate: /etc/kubernetes/pki/front-proxy-client.crt
      client-key: /etc/kubernetes/pki/front-proxy-client.key
current-context: revocation
contexts:
- context:
    cluster: user-authn-revocation
    user: user-authn-revocation
  name: revocation
{{- end }}
