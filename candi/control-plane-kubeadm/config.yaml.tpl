apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
{{- if eq .clusterConfiguration.kubernetesVersion "1.14" }}
kubernetesVersion: 1.14.10
{{- else if eq .clusterConfiguration.kubernetesVersion "1.15" }}
kubernetesVersion: 1.15.11
{{- else if eq .clusterConfiguration.kubernetesVersion "1.16" }}
kubernetesVersion: 1.16.8
{{- else }}
  {{- join (slice "Kubernetes version" .clusterConfiguration.kubernetesVersion "is not supported!") " "| fail }}
{{- end }}
controlPlaneEndpoint: "127.0.0.1:6445"
networking:
  serviceSubnet: {{ .clusterConfiguration.serviceSubnetCIDR | quote }}
  podSubnet: {{ .clusterConfiguration.podSubnetCIDR | quote }}
  dnsDomain: {{ .clusterConfiguration.clusterDomain | quote }}
apiServer:
  extraVolumes:
  - name: "deckhouse-extra-files"
    hostPath: "/etc/kubernetes/deckhouse/extra-files"
    mountPath: "/etc/kubernetes/deckhouse/extra-files"
    readOnly: true
    pathType: DirectoryOrCreate
  extraArgs:
{{- if hasKey . "apiserver" }}
  {{- if hasKey .apiserver "etcdServers" }}
    etcd-servers: {{ .apiserver.etcdServers | join "," | quote }}
  {{- end }}
  {{- if .apiserver.bindToWildcard }}
    bind-address: "0.0.0.0"
  {{- end }}
  {{- if hasKey .apiserver "certSANs" }}
  certSANs:
    {{- range $san := .apiserver.certSANs }}
  - {{ $san | quote }}
    {{- end }}
  {{- end }}
{{- end }}
controllerManager:
  extraVolumes:
  - name: "deckhouse-extra-files"
    hostPath: "/etc/kubernetes/deckhouse/extra-files"
    mountPath: "/etc/kubernetes/deckhouse/extra-files"
    readOnly: true
    pathType: DirectoryOrCreate
  extraArgs:
    node-cidr-mask-size: {{ .clusterConfiguration.podSubnetNodeCIDRPrefix | quote }}
{{- if hasKey . "nodeIP" }}
    bind-address: {{ .nodeIP | quote }}
{{- end }}
    port: "0"
{{- if eq .clusterConfiguration.clusterType "Cloud" }}
    cloud-provider: external
{{- end }}
scheduler:
  extraVolumes:
  - name: "deckhouse-extra-files"
    hostPath: "/etc/kubernetes/deckhouse/extra-files"
    mountPath: "/etc/kubernetes/deckhouse/extra-files"
    readOnly: true
    pathType: DirectoryOrCreate
  extraArgs:
{{- if hasKey . "nodeIP" }}
    bind-address: {{ .nodeIP | quote }}
{{- end }}
    port: "0"
{{- if hasKey . "apiserver" }}
  {{- if hasKey .apiserver "etcdServers" }}
etcd:
  local:
    extraArgs:
      # TODO: We should be able to get rid of this hack and use
      # `kubeadm join phase etcd` after switching to kubeadm
      #  version 1.18+ because it discovers etcd directly from
      #  kubernetes by loking for pods in kube-system, rather
      #  than trying to read endpoints from kubeadm's "cluster
      #  status"
      initial-cluster: {{ .apiserver.etcdServers | join "," | quote }}
      initial-cluster-state: existing
  {{- end }}
{{- end }}
---
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
localAPIEndpoint:
{{- if hasKey . "nodeIP" }}
  advertiseAddress: {{ .nodeIP | quote }}
{{- end }}
  bindPort: 6443
