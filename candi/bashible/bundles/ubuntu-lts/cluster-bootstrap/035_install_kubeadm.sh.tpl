{{ if eq .kubernetesVersion "1.15" }}
  kubernetes_version="1.15.12-00"
{{ else if eq .kubernetesVersion "1.16" }}
  kubernetes_version="1.16.15-00"
{{ else if eq .kubernetesVersion "1.17" }}
  kubernetes_version="1.17.11-00"
{{ else if eq .kubernetesVersion "1.18" }}
  kubernetes_version="1.18.8-00"
{{ else }}
  {{ fail (printf "Unsupported kubernetes version: %s" .kubernetesVersion) }}
{{ end }}

bb-apt-install "kubeadm=$kubernetes_version"
