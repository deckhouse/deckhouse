kubernetes_version=$(cat /var/lib/bashible/kubernetes-version)

if ! apt list --installed kubeadm | grep -F $kubernetes_version; then
  apt-mark unhold kubeadm
  apt install -qy "kubeadm=$kubernetes_version"
fi
