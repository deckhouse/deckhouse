{{ if eq .kubernetesVersion "1.15" }}
  kubernetes_version="1.15.12-0"
{{ else if eq .kubernetesVersion "1.16" }}
  kubernetes_version="1.16.15-0"
{{ else if eq .kubernetesVersion "1.17" }}
  kubernetes_version="1.17.14-0"
{{ else if eq .kubernetesVersion "1.18" }}
  kubernetes_version="1.18.12-0"
{{ else if eq .kubernetesVersion "1.19" }}
  kubernetes_version="1.19.4-0"
{{ else }}
  {{ fail (printf "Unsupported kubernetes version: %s" .kubernetesVersion) }}
{{ end }}

bb-yum-install "kubeadm-$kubernetes_version"
