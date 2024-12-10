{{- define "istio_remote_kubeconfig" }}
{{- $multicluster := index . 0 }}

apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://{{ $multicluster.apiHost }}
    certificate-authority-data: {{ $multicluster.ca | b64enc }}
    insecure-skip-tls-verify: {{ $multicluster.insecureSkipVerify }}
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
