apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
{{- if eq .clusterConfiguration.kubernetesVersion "1.15" }}
kubernetesVersion: 1.15.12
{{- else if eq .clusterConfiguration.kubernetesVersion "1.16" }}
kubernetesVersion: 1.16.15
{{- else if eq .clusterConfiguration.kubernetesVersion "1.17" }}
kubernetesVersion: 1.17.11
{{- else if eq .clusterConfiguration.kubernetesVersion "1.18" }}
kubernetesVersion: 1.18.8
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
    {{- if .apiserver.etcdServers }}
    etcd-servers: >
      https://127.0.0.1:2379,{{ .apiserver.etcdServers | join "," }}
    {{- end }}
  {{- end }}
  {{- if .apiserver.bindToWildcard }}
    bind-address: "0.0.0.0"
  {{- else if .nodeIP }}
    bind-address: {{ .nodeIP | quote }}
  {{- else }}
    bind-address: "0.0.0.0"
  {{- end }}
  {{- if .apiserver.enableDeprecatedAPIs }}
    runtime-config: apps/v1beta1=true,apps/v1beta2=true,extensions/v1beta1/deployments=true,extensions/v1beta1/statefulsets=true,extensions/v1beta1/daemonsets=true,extensions/v1beta1/replicasets=true,extensions/v1beta1/networkpolicies=true,extensions/v1beta1/podsecuritypolicies=true
  {{- end }}
  {{- if .apiserver.oidcCA }}
    oidc-ca-file: /etc/kubernetes/deckhouse/extra-files/oidc-ca.crt
  {{- end }}
  {{- if .apiserver.oidcIssuerURL }}
    oidc-client-id: kubernetes
    oidc-groups-claim: groups
    oidc-username-claim: email
    oidc-issuer-url: {{ .apiserver.oidcIssuerURL }}
  {{- end }}
  {{ if .apiserver.webhookURL }}
    authorization-mode: Node,Webhook,RBAC
    authorization-webhook-config-file: /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml
  {{- end -}}
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
    v: "8"
{{- if hasKey . "etcd" }}
  {{- if hasKey .etcd "existingCluster" }}
    {{- if .etcd.existingCluster }}
etcd:
  local:
    extraArgs:
      # without this parameter, when restarting etcd, and /var/lib/etcd/member does not exist, it'll start with a new empty cluster
      initial-cluster-state: existing
    {{- end }}
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
---
apiVersion: kubeadm.k8s.io/v1beta2
kind: JoinConfiguration
discovery:
  file:
    kubeConfigPath: "/etc/kubernetes/admin.conf"
controlPlane:
  localAPIEndpoint:
{{- if hasKey . "nodeIP" }}
    advertiseAddress: {{ .nodeIP | quote }}
{{- end }}
    bindPort: 6443
