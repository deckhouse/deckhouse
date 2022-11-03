# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

{{- $kubernetesVersion := printf "%s%s" (.kubernetesVersion | toString) (index .k8s .kubernetesVersion "patch" | toString) | replace "." "" }}
{{- $kubernetesCniVersion := index .k8s .kubernetesVersion "cniVersion" | toString | replace "." "" }}
bb-rp-remove kubeadm
bb-rp-install "kubernetes-cni:{{ index .images.registrypackages (printf "kubernetesCniAlteros%s" $kubernetesCniVersion) | toString }}" "kubectl:{{ index .images.registrypackages (printf "kubectlAlteros%s" $kubernetesVersion) | toString }}"

old_kubelet_hash=""
if [ -f "${BB_RP_INSTALLED_PACKAGES_STORE}/kubelet/tag" ]; then
  old_kubelet_hash=$(<"${BB_RP_INSTALLED_PACKAGES_STORE}/kubelet/tag")
fi

bb-rp-install "kubelet:{{ index .images.registrypackages (printf "kubeletAlteros%s" $kubernetesVersion) | toString }}"

new_kubelet_hash=$(<"${BB_RP_INSTALLED_PACKAGES_STORE}/kubelet/tag")
if [[ "${old_kubelet_hash}" != "${new_kubelet_hash}" ]]; then
  bb-flag-set kubelet-need-restart
fi

mkdir -p /etc/kubernetes/manifests
mkdir -p /etc/systemd/system/kubelet.service.d
mkdir -p /etc/kubernetes/pki
mkdir -p /var/lib/kubelet
