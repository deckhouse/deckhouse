package template

const staticTemplate = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: {{ .cluster_state.pod_subnet_cidr }}
serviceSubnetCIDR: {{ .cluster_state.pod_subnet_cidr }}
kubernetesVersion: {{ .cluster_state.k8s_version | quote }}
clusterDomain: {{ .cluster_state.cluster_domain | quote }}
podSubnetNodeCIDRPrefix: {{ .cluster_state.subnet_node_cidr_prefix | quote }}
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: {{ .registry_state.repo | quote}}
  registryDockerCfg: {{ .registry_state.dockerconf | quote}}
{{- if .registry_state.ca }}
  registryCA: {{ .registry_state.ca | quote }}
{{- end }}
  registryScheme: {{ .registry_state.schema | quote }}
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Default
    releaseChannel: {{ .deckhouse_state.release_channel | quote }}
    logLevel: Info
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    modules:
      publicDomainTemplate: {{ .deckhouse_state.public_domain_template | quote }}

{{- if .deckhouse_state.enable_publish_k8s_api }}
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 1
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: DoNotNeed
    publishAPI:
      enable: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""

{{- end }}
---
{{- if eq .cni_state.cni_type "Cilium" }}
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  version: 1
  enabled: true
  settings:
    tunnelMode: VXLAN
{{- else if eq .cni_state.cni_type "Flannel" }}
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-flannel
spec:
  version: 1
  enabled: true
  settings:
    podNetworkMode: {{ .cni_state.flannel_mode }}
{{- end }}
---
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- {{ .static_state.internal_network_cidr | quote }}
`
