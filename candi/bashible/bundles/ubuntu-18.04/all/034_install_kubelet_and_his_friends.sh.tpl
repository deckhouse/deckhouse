if bb-flag? is-bootstrapped; then exit 0; fi

kubernetes_version="$(cat /var/lib/bashible/kubernetes-version)"
cni_version=0.7.5-00

if ! apt list --installed kubelet | grep -F $kubernetes_version; then
  apt-mark unhold kubelet kubectl kubernetes-cni
  apt install -qy "kubelet=${kubernetes_version}" "kubectl=${kubernetes_version}" "kubernetes-cni=${cni_version}"
  apt-mark hold kubelet kubectl kubernetes-cni
fi

mkdir -p /etc/kubernetes/manifests
mkdir -p /etc/systemd/system/kubelet.service.d
mkdir -p /etc/kubernetes/pki
