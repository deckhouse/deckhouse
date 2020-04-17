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
  extraArgs:
{{- if has .extraArgs "apiServer" }}
{{ .extraArgs.apiServer | toYAML | indent 6 }}
{{- end }}
#    bind-address: "0.0.0.0"
#  certSANs:
#  - "blah-blah-blah.com"
#  - "trum-pum-pum.com"
controllerManager:
  extraArgs:
    node-cidr-mask-size: {{ .clusterConfiguration.podSubnetNodeCIDRPrefix | quote }}
    #bind-address: {{ .nodeIP }}
    port: "0"
{{- if eq .clusterConfiguration.clusterType "Cloud" }}
    cloud-provider: external
{{- else }}
  {{- join (slice "Cluster type version" .clusterConfiguration.clusterType "is not supported!") " "| fail }}
{{- end }}
{{- if has .extraArgs "controllerManager" }}
{{ .extraArgs.controllerManager | toYAML | indent 6 }}
{{- end }}
scheduler:
  extraArgs:
    #bind-address: {{ .nodeIP }}
    port: "0"
{{- if has .extraArgs "scheduler" }}
{{ .extraArgs.scheduler | toYAML | indent 6 }}
{{- end }}
{{- if has .extraArgs "etcd" }}
etcd:
  local:
    extraArgs:
{{ .extraArgs.etcd | toYAML | indent 6 }}
{{- end }}
---
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: {{ .nodeIP }}
  bindPort: 6443
