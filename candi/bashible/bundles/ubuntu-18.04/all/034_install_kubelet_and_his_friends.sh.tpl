{{ if eq .kubernetesVersion "1.14" }}
kubernetes_version="1.14.10-00"
{{ else if eq .kubernetesVersion "1.15" }}
kubernetes_version="1.15.11-00"
{{ else if eq .kubernetesVersion "1.16" }}
kubernetes_version="1.16.8-00"
{{ else }}
  {{ fail (printf "Unsupported kubernetes version: %s" .kubernetesVersion) }}
{{ end }}
cni_version=0.7.5-00

bb-apt-install "kubelet=${kubernetes_version}" "kubectl=${kubernetes_version}" "kubernetes-cni=${cni_version}"

mkdir -p /etc/kubernetes/manifests
mkdir -p /etc/systemd/system/kubelet.service.d
mkdir -p /etc/kubernetes/pki
