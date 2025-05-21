{{- /*
RotateKubeletServerCertificate default is true, but CIS benchmark wants it to be explicitly enabled
https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/
*/ -}}
{{- $featureGates := list "TopologyAwareHints=true" "RotateKubeletServerCertificate=true" | join "," -}}
{{- /* admissionPlugins */ -}}
{{- $admissionPlugins := list "NodeRestriction" "PodNodeSelector" "PodTolerationRestriction" "EventRateLimit" "ExtendedResourceToleration" -}}
{{- if .apiserver.admissionPlugins -}}
  {{ $admissionPlugins = concat $admissionPlugins .apiserver.admissionPlugins | uniq -}}
{{- end -}}
{{- /* sa issuers and audiences */ -}}
{{- $sa := .apiserver.serviceAccount | default dict -}}
{{- $primaryAud := $sa.issuer }}
{{- $defaultAud := printf "https://kubernetes.default.svc.%s" .clusterConfiguration.clusterDomain -}}
{{- $audiences := list $primaryAud -}}
{{- if $sa.additionalAPIIssuers -}}
  {{- range $sa.additionalAPIIssuers -}}
    {{- if and (ne . $primaryAud) (ne . $defaultAud) -}}
      {{- $audiences = append $audiences . -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- if $sa.additionalAPIAudiences -}}
  {{- range $sa.additionalAPIAudiences -}}
    {{- if and (ne . $primaryAud) (ne . $defaultAud) -}}
      {{- $audiences = append $audiences . -}}
    {{- end }}
  {{- end }}
{{- end }}
{{- $audiences = $audiences | uniq -}}
{{- $audiences = without $audiences $defaultAud -}}
{{- $audiences = append $audiences $defaultAud -}}
{{- /* ClusterConfiguration */ -}}
apiVersion: kubeadm.k8s.io/v1beta4
kind: ClusterConfiguration
kubernetesVersion: {{ printf "%s.%s" (.clusterConfiguration.kubernetesVersion | toString) (index .k8s .clusterConfiguration.kubernetesVersion "patch" | toString) }}
clusterName: {{ .clusterConfiguration.clusterName | default "kubernetes" }}
controlPlaneEndpoint: "127.0.0.1:6445"
certificatesDir: /etc/kubernetes/pki
certificateValidityPeriod: 8760h0m0s
caCertificateValidityPeriod: 87600h0m0s
imageRepository: {{ .clusterConfiguration.imageRepository | default "registry.k8s.io" }}
encryptionAlgorithm: {{ .clusterConfiguration.encryptionAlgorithm }}
networking:
  serviceSubnet: {{ .clusterConfiguration.serviceSubnetCIDR | quote }}
  podSubnet: {{ .clusterConfiguration.podSubnetCIDR | quote }}
  dnsDomain: {{ .clusterConfiguration.clusterDomain | quote }}
apiServer:
  extraVolumes:
    - name: deckhouse-extra-files
      hostPath: /etc/kubernetes/deckhouse/extra-files
      mountPath: /etc/kubernetes/deckhouse/extra-files
      readOnly: true
      pathType: DirectoryOrCreate
    - name: etc-pki
      hostPath: /etc/pki
      mountPath: /etc/pki
      readOnly: true
      pathType: DirectoryOrCreate
    {{- if .apiserver.auditPolicy }}
    {{- if eq .apiserver.auditLog.output "File" }}
    - name: kube-audit-log
      hostPath: "{{ .apiserver.auditLog.path }}"
      mountPath: "{{ .apiserver.auditLog.path }}"
      readOnly: false
      pathType: DirectoryOrCreate
    {{- end }}
    {{- end }}
  extraArgs:
    - name: anonymous-auth
      value: "false"
    - name: api-audiences
      value: {{ $audiences | join "," }}
    - name: service-account-issuer
      value: {{ if $sa.issuer }}{{ $sa.issuer }}{{ else }}https://kubernetes.default.svc.{{ .clusterConfiguration.clusterDomain }}{{ end }}
    - name: service-account-jwks-uri
      value: {{ if $sa.issuer }}{{ $sa.issuer }}/openid/v1/jwks{{ else }}https://kubernetes.default.svc.{{ .clusterConfiguration.clusterDomain }}/openid/v1/jwks{{ end }}
    - name: service-account-key-file
      value: /etc/kubernetes/pki/sa.pub
    - name: service-account-signing-key-file
      value: /etc/kubernetes/pki/sa.key
    {{- if ne .runType "ClusterBootstrap" }}
    - name: enable-admission-plugins
      value: "{{ $admissionPlugins | sortAlpha | join "," }}"
    - name: admission-control-config-file
      value: /etc/kubernetes/deckhouse/extra-files/admission-control-config.yaml
    - name: kubelet-certificate-authority
      value: /etc/kubernetes/pki/ca.crt
    {{- end }}
    {{- if .apiserver.auditPolicy }}
    - name: audit-policy-file
      value: /etc/kubernetes/deckhouse/extra-files/audit-policy.yaml
    - name: audit-log-format
      value: json
    {{- if eq .apiserver.auditLog.output "File" }}
    - name: audit-log-path
      value: "{{ .apiserver.auditLog.path }}/audit.log"
    - name: audit-log-truncate-enabled
      value: "true"
    - name: audit-log-maxage
      value: "7"
    - name: audit-log-maxsize
      value: "100"
    - name: audit-log-maxbackup
      value: "10"
    {{- else }}
    - name: audit-log-path
      value: "-"
    {{- end }}
    {{- end }}
    - name: bind-address
      value: {{ if .apiserver.bindToWildcard }}"0.0.0.0"{{ else if .nodeIP }}{{ .nodeIP | quote }}{{ else }}"0.0.0.0"{{ end }}
    {{- if hasKey . "arguments" }}
      {{- if hasKey .arguments "defaultUnreachableTolerationSeconds" }}
    - name: default-unreachable-toleration-seconds
      value: {{ .arguments.defaultUnreachableTolerationSeconds | quote }}
      {{- end }}
      {{- if hasKey .arguments "podEvictionTimeout" }}
    - name: default-not-ready-toleration-seconds
      value: {{ .arguments.podEvictionTimeout | quote }}
      {{- end }}
    {{- end }}
    - name: etcd-servers
      value: >-
        https://127.0.0.1:2379{{ if .apiserver.etcdServers }},{{ .apiserver.etcdServers | join "," }}{{ end }}
    - name: feature-gates
      value: {{ $featureGates | quote }}
    {{- if semverCompare ">= 1.28" .clusterConfiguration.kubernetesVersion }}
    - name: runtime-config
      value: admissionregistration.k8s.io/v1beta1=true,admissionregistration.k8s.io/v1alpha1=true
    {{- end }}
    {{- if .apiserver.oidcCA }}
    - name: oidc-ca-file
      value: /etc/kubernetes/deckhouse/extra-files/oidc-ca.crt
    {{- end }}
    {{- if .apiserver.oidcIssuerURL }}
    - name: oidc-client-id
      value: kubernetes
    - name: oidc-groups-claim
      value: groups
    - name: oidc-username-claim
      value: email
    - name: oidc-issuer-url
      value: {{ .apiserver.oidcIssuerURL }}
    {{- end }}
    - name: profiling
      value: "false"
    - name: request-timeout
      value: 60s
    - name: tls-cipher-suites
      value: TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256
    {{- if hasKey .apiserver "certSANs" }}
    certSANs:
      {{- range $san := .apiserver.certSANs }}
      - {{ $san | quote }}
      {{- end }}
    {{- end }}
controllerManager:
  extraVolumes:
    - name: deckhouse-extra-files
      hostPath: /etc/kubernetes/deckhouse/extra-files
      mountPath: /etc/kubernetes/deckhouse/extra-files
      readOnly: true
      pathType: DirectoryOrCreate
  extraArgs:
    - name: profiling
      value: "false"
    - name: terminated-pod-gc-threshold
      value: "12500"
    - name: feature-gates
      value: {{ $featureGates | quote }}
    - name: node-cidr-mask-size
      value: {{ .clusterConfiguration.podSubnetNodeCIDRPrefix | quote }}
    - name: bind-address
      value: "127.0.0.1"
    {{- if eq .clusterConfiguration.clusterType "Cloud" }}
    - name: cloud-provider
      value: external
    {{- end }}
    {{- if hasKey . "arguments" }}
      {{- if hasKey .arguments "nodeMonitorPeriod" }}
    - name: node-monitor-period
      value: "{{ .arguments.nodeMonitorPeriod }}s"
    - name: node-monitor-grace-period
      value: "{{ .arguments.nodeMonitorGracePeriod }}s"
      {{- end }}
    {{- end }}
scheduler:
  extraVolumes:
    - name: deckhouse-extra-files
      hostPath: /etc/kubernetes/deckhouse/extra-files
      mountPath: /etc/kubernetes/deckhouse/extra-files
      readOnly: true
      pathType: DirectoryOrCreate
  extraArgs:
    - name: profiling
      value: "false"
    - name: feature-gates
      value: {{ $featureGates | quote }}
    - name: bind-address
      value: "127.0.0.1"
    {{- if ne .runType "ClusterBootstrap" }}
    - name: config
      value: /etc/kubernetes/deckhouse/extra-files/scheduler-config.yaml
    {{- end }}
{{- if hasKey . "etcd" }}
  {{- if hasKey .etcd "existingCluster" }}
    {{- if .etcd.existingCluster }}
etcd:
  local:
    extraArgs:
      - name: initial-cluster-state
        value: existing
      - name: experimental-initial-corrupt-check
        value: "true"
      {{- if hasKey .etcd "quotaBackendBytes" }}
      - name: quota-backend-bytes
        value: {{ .etcd.quotaBackendBytes | quote }}
      - name: metrics
        value: extensive
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
---
apiVersion: kubeadm.k8s.io/v1beta4
kind: InitConfiguration
localAPIEndpoint:
  {{- if hasKey . "nodeIP" }}
  advertiseAddress: {{ .nodeIP | quote }}
  {{- end }}
  bindPort: 6443
patches:
  directory: /etc/kubernetes/deckhouse/kubeadm/patches/
---
apiVersion: kubeadm.k8s.io/v1beta4
kind: JoinConfiguration
caCertPath: /etc/kubernetes/pki/ca.crt
discovery:
  file:
    kubeConfigPath: /etc/kubernetes/admin.conf
controlPlane:
  localAPIEndpoint:
    {{- if hasKey . "nodeIP" }}
    advertiseAddress: {{ .nodeIP | quote }}
    {{- end }}
    bindPort: 6443
patches:
  directory: /etc/kubernetes/deckhouse/kubeadm/patches/
