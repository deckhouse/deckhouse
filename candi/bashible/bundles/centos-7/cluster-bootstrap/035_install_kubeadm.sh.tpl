kubernetes_version="{{ printf "%s.%s-0" (.kubernetesVersion | toString) (index .k8s .kubernetesVersion "patch" | toString) }}"

bb-yum-install "kubeadm-$kubernetes_version"
