if bb-flag? is-bootstrapped; then exit 0; fi

{{ if eq .kubernetesVersion "1.14" }}
  kubernetes_version="1.14.10"
{{ else if eq .kubernetesVersion "1.15" }}
  kubernetes_version="1.15.11"
{{ else if eq .kubernetesVersion "1.16" }}
  kubernetes_version="1.16.8"
{{ else }}
  {{ fail (printf "Unsupported kubernetes version: %s" .kubernetesVersion) }}
{{ end }}

if ! yum list installed kubelet | grep -F $kubernetes_version; then
  yum install -y "kubelet-$kubernetes_version" "kubectl-$kubernetes_version"
  yum versionlock "kubelet-*" "kubectl-*" "kubernetes-cni-*"
fi
