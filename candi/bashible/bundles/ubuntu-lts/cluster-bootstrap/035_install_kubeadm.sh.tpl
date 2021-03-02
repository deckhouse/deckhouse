kubernetes_version="{{ printf "%s.%s-00" (.kubernetesVersion | toString) (index .k8s .kubernetesVersion "patch" | toString) }}"

bb-apt-install "kubeadm=$kubernetes_version"
