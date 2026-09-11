{{- define "istio_remote_kubeconfig" }}
{{- $multicluster := index . 0 }}

apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://{{ $multicluster.apiHost }}
    certificate-authority-data: {{ $multicluster.metadataExporterCA | b64enc }}
    insecure-skip-tls-verify: {{ $multicluster.insecureSkipVerify }}
  name: {{ $multicluster.clusterID }}
contexts:
- context:
    cluster: {{ $multicluster.clusterID }}
    user: {{ $multicluster.clusterID }}
  name: {{ $multicluster.clusterID }}
current-context: {{ $multicluster.clusterID }}
preferences: {}
users:
- name: {{ $multicluster.clusterID }}
  user:
    token: {{ $multicluster.apiJWT }}
{{- end }}
