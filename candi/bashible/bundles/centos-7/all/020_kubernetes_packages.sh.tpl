{{ if eq .kubernetesVersion "1.14" }}
  kubernetes_version="1.14.10"
{{ else if eq .kubernetesVersion "1.15" }}
  kubernetes_version="1.15.11"
{{ else if eq .kubernetesVersion "1.16" }}
  kubernetes_version="1.16.8"
{{ else }}
  {{ fail (printf "Unsupported kubernetes version: %s" .kubernetesVersion) }}
{{ end }}

bb-yum-install "kubelet-$kubernetes_version" "kubectl-$kubernetes_version"
