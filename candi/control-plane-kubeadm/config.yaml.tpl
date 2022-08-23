{{- $featureGates := "" }}
{{- if semverCompare ">= 1.21" .clusterConfiguration.kubernetesVersion }}
    {{- $featureGates = "EndpointSliceTerminatingCondition=true,DaemonSetUpdateSurge=true" }}
{{- end }}
{{- if semverCompare "= 1.20" .clusterConfiguration.kubernetesVersion }}
    {{- $featureGates = "EndpointSliceTerminatingCondition=true,TTLAfterFinished=true" }}
{{- end }}
{{- if semverCompare "< 1.20" .clusterConfiguration.kubernetesVersion }}
    {{- $featureGates = "TTLAfterFinished=true" }}
{{- end }}

{{- if semverCompare ">= 1.22" .clusterConfiguration.kubernetesVersion }}
apiVersion: kubeadm.k8s.io/v1beta3
{{- else }}
apiVersion: kubeadm.k8s.io/v1beta2
{{- end }}
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
  - name: "etc-pki"
    hostPath: "/etc/pki"
    mountPath: "/etc/pki"
    readOnly: true
    pathType: DirectoryOrCreate
{{- if .apiserver.auditPolicy }}
  {{- if eq .apiserver.auditLog.output "File" }}
  - name: "kube-audit-log"
    hostPath: "{{ .apiserver.auditLog.path }}"
    mountPath: "{{ .apiserver.auditLog.path }}"
    readOnly: false
    pathType: DirectoryOrCreate
  {{- end }}
{{- end }}
  extraArgs:
{{- if .apiserver.serviceAccount }}
    api-audiences: https://kubernetes.default.svc.{{ .clusterConfiguration.clusterDomain }}{{ with .apiserver.serviceAccount.additionalAPIAudiences }},{{ . | join "," }}{{ end }}
    service-account-issuer: https://kubernetes.default.svc.{{ .clusterConfiguration.clusterDomain }}
    service-account-key-file: /etc/kubernetes/pki/sa.pub
    service-account-signing-key-file: /etc/kubernetes/pki/sa.key
{{- end }}
{{- if ne .runType "ClusterBootstrap" }}
    enable-admission-plugins: "EventRateLimit,ExtendedResourceToleration{{ if .apiserver.admissionPlugins }},{{ .apiserver.admissionPlugins | join "," }}{{ end }}"
    admission-control-config-file: "/etc/kubernetes/deckhouse/extra-files/admission-control-config.yaml"
# kubelet-certificate-authority flag should be set after bootstrap of first master.
# This flag affects logs from kubelets, for period of time between kubelet start and certificate request approve by Deckhouse hook.
    kubelet-certificate-authority: "/etc/kubernetes/pki/ca.crt"
{{- end }}
    anonymous-auth: "false"
    feature-gates: {{ $featureGates | quote }}
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
  {{ if .apiserver.authnWebhookURL }}
    authentication-token-webhook-config-file: /etc/kubernetes/deckhouse/extra-files/authn-webhook-config.yaml
  {{- end -}}
  {{ if .apiserver.authnWebhookCacheTTL }}
    authentication-token-webhook-cache-ttl: {{.apiserver.authnWebhookCacheTTL | quote }}
  {{- end -}}
  {{- if .apiserver.auditPolicy }}
    audit-policy-file: /etc/kubernetes/deckhouse/extra-files/audit-policy.yaml
    audit-log-format: json
    {{- if eq .apiserver.auditLog.output "File" }}
    audit-log-path: "{{ .apiserver.auditLog.path }}/audit.log"
    audit-log-truncate-enabled: "true"
    audit-log-maxage: "7"
    audit-log-maxsize: "100"
    audit-log-maxbackup: "10"
    {{- else }}
    audit-log-path: "-"
    {{- end }}
  {{- end }}
  {{- if .apiserver.secretEncryptionKey }}
    encryption-provider-config: /etc/kubernetes/deckhouse/extra-files/secret-encryption-config.yaml
  {{- end }}
    profiling: "false"
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
    profiling: "false"
    terminated-pod-gc-threshold: "12500"
    feature-gates: {{ $featureGates | quote }}
    node-cidr-mask-size: {{ .clusterConfiguration.podSubnetNodeCIDRPrefix | quote }}
    bind-address: "127.0.0.1"
{{- if semverCompare "< 1.24" .clusterConfiguration.kubernetesVersion }}
    port: "0"
{{- end }}
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
{{- if ne .runType "ClusterBootstrap" }}
    config: "/etc/kubernetes/deckhouse/extra-files/scheduler-config.yaml"
{{- end }}
    profiling: "false"
{{- if semverCompare "< 1.20" .clusterConfiguration.kubernetesVersion }}
    feature-gates: "DefaultPodTopologySpread=true"
{{- else }}
    feature-gates: {{ $featureGates | quote }}
{{- end }}
    bind-address: "127.0.0.1"
{{- if semverCompare "< 1.24" .clusterConfiguration.kubernetesVersion }}
    port: "0"
{{- end }}
{{- if hasKey . "etcd" }}
  {{- if hasKey .etcd "existingCluster" }}
    {{- if .etcd.existingCluster }}
etcd:
  local:
    extraArgs:
      # without this parameter, when restarting etcd, and /var/lib/etcd/member does not exist, it'll start with a new empty cluster
      initial-cluster-state: existing
      experimental-initial-corrupt-check: "true"
      {{- if hasKey .etcd "quotaBackendBytes" }}
      quota-backend-bytes: {{ .etcd.quotaBackendBytes | quote }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
---
{{- if semverCompare ">= 1.22" .clusterConfiguration.kubernetesVersion }}
apiVersion: kubeadm.k8s.io/v1beta3
{{- else }}
apiVersion: kubeadm.k8s.io/v1beta2
{{- end }}
kind: InitConfiguration
{{- if semverCompare ">= 1.22" .clusterConfiguration.kubernetesVersion }}
patches:
  directory: /etc/kubernetes/deckhouse/kubeadm/patches/
{{- end }}
localAPIEndpoint:
{{- if hasKey . "nodeIP" }}
  advertiseAddress: {{ .nodeIP | quote }}
{{- end }}
  bindPort: 6443
---
{{- if semverCompare ">= 1.22" .clusterConfiguration.kubernetesVersion }}
apiVersion: kubeadm.k8s.io/v1beta3
{{- else }}
apiVersion: kubeadm.k8s.io/v1beta2
{{- end }}
kind: JoinConfiguration
{{- if semverCompare ">= 1.22" .clusterConfiguration.kubernetesVersion }}
patches:
  directory: /etc/kubernetes/deckhouse/kubeadm/patches/
{{- end }}
discovery:
  file:
    kubeConfigPath: "/etc/kubernetes/admin.conf"
controlPlane:
  localAPIEndpoint:
{{- if hasKey . "nodeIP" }}
    advertiseAddress: {{ .nodeIP | quote }}
{{- end }}
    bindPort: 6443
