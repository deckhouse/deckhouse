apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
kubernetesVersion: {{ printf "%s.%s" (.clusterConfiguration.kubernetesVersion | toString ) (index .k8s .clusterConfiguration.kubernetesVersion "patch" | toString) }}
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
{{- if .apiserver.auditPolicy }}
  - name: "kube-audit-log"
    hostPath: "/var/log/kube-audit"
    mountPath: "/var/log/kube-audit"
    readOnly: false
    pathType: DirectoryOrCreate
{{- end }}
  extraArgs:
    feature-gates: "EndpointSliceTerminatingCondition=true"
{{- if hasKey . "arguments" }}
  {{- if hasKey .arguments "defaultUnreachableTolerationSeconds" }}
    default-unreachable-toleration-seconds: {{ .arguments.defaultUnreachableTolerationSeconds | quote }}
  {{- end }}
{{- end }}
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
  {{- if .apiserver.auditPolicy }}
    audit-policy-file: /etc/kubernetes/deckhouse/extra-files/audit-policy.yaml
    audit-log-path: /var/log/kube-audit/audit.log
    audit-log-format: json
    audit-log-truncate-enabled: "true"
    audit-log-maxage: "7"
    audit-log-maxsize: "100"
    audit-log-maxbackup: "10"
  {{- end }}
    profiling: "true"
    request-timeout: "300s"
    tls-cipher-suites: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
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
    profiling: "true"
    terminated-pod-gc-threshold: "10"
    feature-gates: "EndpointSliceTerminatingCondition=true"
    node-cidr-mask-size: {{ .clusterConfiguration.podSubnetNodeCIDRPrefix | quote }}
{{- if hasKey . "nodeIP" }}
    bind-address: {{ .nodeIP | quote }}
{{- end }}
    port: "0"
{{- if eq .clusterConfiguration.clusterType "Cloud" }}
    cloud-provider: external
{{- end }}
{{- if hasKey . "arguments" }}
  {{- if hasKey .arguments "nodeMonitorPeriod" }}
    node-monitor-period: "{{ .arguments.nodeMonitorPeriod }}s"
    node-monitor-grace-period: "{{ .arguments.nodeMonitorGracePeriod }}s"
  {{- end }}
  {{- if hasKey .arguments "podEvictionTimeout" }}
    pod-eviction-timeout: "{{ .arguments.podEvictionTimeout }}s"
  {{- end }}
{{- end }}
scheduler:
  extraVolumes:
  - name: "deckhouse-extra-files"
    hostPath: "/etc/kubernetes/deckhouse/extra-files"
    mountPath: "/etc/kubernetes/deckhouse/extra-files"
    readOnly: true
    pathType: DirectoryOrCreate
  extraArgs:
    profiling: "true"
    feature-gates: "EndpointSliceTerminatingCondition=true"
{{- if hasKey . "nodeIP" }}
    bind-address: {{ .nodeIP | quote }}
{{- end }}
    port: "0"
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
