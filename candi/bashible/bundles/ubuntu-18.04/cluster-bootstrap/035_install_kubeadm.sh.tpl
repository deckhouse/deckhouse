{{ if eq .kubernetesVersion "1.15" }}
  kubernetes_version="1.16.8-00"
{{ else if eq .kubernetesVersion "1.16" }}
  kubernetes_version="1.16.8-00"
{{ else }}
  {{ fail (printf "Unsupported kubernetes version: %s" .kubernetesVersion) }}
{{ end }}

bb-apt-install "kubeadm=$kubernetes_version"
