{{ if eq .kubernetesVersion "1.14" }}
  kubernetes_version="1.14.10"
{{ else if eq .kubernetesVersion "1.15" }}
  kubernetes_version="1.15.12"
{{ else if eq .kubernetesVersion "1.16" }}
  kubernetes_version="1.16.10"
{{ else if eq .kubernetesVersion "1.17" }}
  kubernetes_version="1.17.6"
{{ else if eq .kubernetesVersion "1.18" }}
  kubernetes_version="1.18.3"
{{ else }}
  {{ fail (printf "Unsupported kubernetes version: %s" .kubernetesVersion) }}
{{ end }}

bb-yum-install "kubelet-$kubernetes_version" "kubectl-$kubernetes_version"
