{{/*
RotateKubeletServerCertificate default is true, but CIS becnhmark wants it to be explicitly enabled
https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/
*/}}
{{- $featureGates := list "TopologyAwareHints=true" "RotateKubeletServerCertificate=true" | join "," }}
{{- if semverCompare ">= 1.26" .clusterConfiguration.kubernetesVersion }}
    {{- $featureGates = list $featureGates "ValidatingAdmissionPolicy=true" | join "," }}
{{- end }}
{{- if semverCompare "< 1.27" .clusterConfiguration.kubernetesVersion }}
    {{- $featureGates = list $featureGates "DaemonSetUpdateSurge=true" | join "," }}
{{- end }}
{{- if semverCompare "< 1.28" .clusterConfiguration.kubernetesVersion }}
    {{- $featureGates = list $featureGates "EndpointSliceTerminatingCondition=true" | join "," }}
    {{- $featureGates = list $featureGates "InTreePluginRBDUnregister=true" | join "," }}
{{- end }}

apiVersion: kubeadm.k8s.io/v1beta3
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
    {{- if .apiserver.serviceAccount.issuer }}
    service-account-issuer: {{ .apiserver.serviceAccount.issuer }}
    {{- else }}
    service-account-issuer: https://kubernetes.default.svc.{{ .clusterConfiguration.clusterDomain }}
    {{- end }}
    service-account-key-file: /etc/kubernetes/pki/sa.pub
    service-account-signing-key-file: /etc/kubernetes/pki/sa.key
{{- end }}
{{- if ne .runType "ClusterBootstrap" }}
    {{ $admissionPlugins := list "NodeRestriction" "PodNodeSelector" "PodTolerationRestriction" "EventRateLimit" "ExtendedResourceToleration" }}
    {{- if .apiserver.admissionPlugins }}
      {{ $admissionPlugins = concat $admissionPlugins .apiserver.admissionPlugins | uniq }}
    {{- end }}
    enable-admission-plugins: "{{ $admissionPlugins | sortAlpha | join "," }}"
    admission-control-config-file: "/etc/kubernetes/deckhouse/extra-files/admission-control-config.yaml"
# kubelet-certificate-authority flag should be set after bootstrap of first master.
# This flag affects logs from kubelets, for period of time between kubelet start and certificate request approve by Deckhouse hook.
    kubelet-certificate-authority: "/etc/kubernetes/pki/ca.crt"
{{- end }}
    anonymous-auth: "false"
    feature-gates: {{ $featureGates | quote }}
{{- if semverCompare ">= 1.28" .clusterConfiguration.kubernetesVersion }}
    runtime-config: "admissionregistration.k8s.io/v1beta1=true"
{{- else if semverCompare ">= 1.26" .clusterConfiguration.kubernetesVersion }}
    runtime-config: "admissionregistration.k8s.io/v1alpha1=true"
{{- end }}
{{- if hasKey . "arguments" }}
  {{- if hasKey .arguments "defaultUnreachableTolerationSeconds" }}
    default-unreachable-toleration-seconds: {{ .arguments.defaultUnreachableTolerationSeconds | quote }}
  {{- end }}
  {{- if and (hasKey .arguments "podEvictionTimeout") (semverCompare "> 1.26" .clusterConfiguration.kubernetesVersion) }}
    default-not-ready-toleration-seconds: "{{ .arguments.podEvictionTimeout }}"
    default-unreachable-toleration-seconds: "{{ .arguments.podEvictionTimeout }}"
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
  {{ if .apiserver.auditWebhookURL }}
    audit-webhook-config-file: /etc/kubernetes/deckhouse/extra-files/audit-webhook-config.yaml
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
{{- if eq .clusterConfiguration.clusterType "Cloud" }}
    cloud-provider: external
{{- end }}
{{- if hasKey . "arguments" }}
  {{- if hasKey .arguments "nodeMonitorPeriod" }}
    node-monitor-period: "{{ .arguments.nodeMonitorPeriod }}s"
    node-monitor-grace-period: "{{ .arguments.nodeMonitorGracePeriod }}s"
  {{- end }}
  {{- if and (hasKey .arguments "podEvictionTimeout") (semverCompare "< 1.27" .clusterConfiguration.kubernetesVersion) }}
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
    feature-gates: {{ $featureGates | quote }}
    bind-address: "127.0.0.1"
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
      metrics: extensive
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
patches:
  directory: /etc/kubernetes/deckhouse/kubeadm/patches/
localAPIEndpoint:
{{- if hasKey . "nodeIP" }}
  advertiseAddress: {{ .nodeIP | quote }}
{{- end }}
  bindPort: 6443
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: JoinConfiguration
patches:
  directory: /etc/kubernetes/deckhouse/kubeadm/patches/
discovery:
  file:
    kubeConfigPath: "/etc/kubernetes/admin.conf"
controlPlane:
  localAPIEndpoint:
{{- if hasKey . "nodeIP" }}
    advertiseAddress: {{ .nodeIP | quote }}
{{- end }}
    bindPort: 6443
