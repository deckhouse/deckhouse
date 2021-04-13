{{- define "istio_remote_kubeconfig" }}
{{- $multicluster := index . 0 }}
{{- $clientCertificate := index . 1 }}

apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://{{ $multicluster.apiHost }}
  name: {{ $multicluster.name }}
contexts:
- context:
    cluster: {{ $multicluster.name }}
    user: {{ $multicluster.name }}
  name: {{ $multicluster.name }}
current-context: {{ $multicluster.name }}
preferences: {}
users:
- name: {{ $multicluster.name }}
  user:
    client-certificate-data: {{ $clientCertificate.cert | b64enc }}
    client-key-data: {{ $clientCertificate.key | b64enc }}
{{- end }}
