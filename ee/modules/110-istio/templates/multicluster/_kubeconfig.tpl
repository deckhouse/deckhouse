{{- define "istio_remote_kubeconfig" }}
{{- $multicluster := index . 0 }}

apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://{{ $multicluster.apiHost }}
    certificate-authority-data: {{ $multicluster.metadataCA | b64enc }}
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
    token: {{ $multicluster.apiJWT }}
{{- end }}
