# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

{{- $kubernetesVersion := printf "%s%s" (.kubernetesVersion | toString) (index .k8s .kubernetesVersion "patch" | toString) | replace "." "" }}
{{- $kubernetesMajorVersion := .kubernetesVersion | toString | replace "." "" }}
{{- $kubernetesCniVersion := index .k8s .kubernetesVersion "cniVersion" | toString | replace "." "" }}

bb-rp-install "kubeadm:{{ index .images.registrypackages (printf "kubeadmDebian%s" $kubernetesVersion) }}" "kubelet:{{ index .images.registrypackages (printf "kubeletDebian%s" $kubernetesVersion) }}" "kubectl:{{ index .images.registrypackages (printf "kubectlDebian%s" $kubernetesVersion) }}" "crictl:{{ index .images.registrypackages (printf "crictl%s" $kubernetesMajorVersion) }}" "kubernetes-cni:{{ index .images.registrypackages (printf "kubernetesCniDebian%s" $kubernetesCniVersion) }}"
