{{- $baseFeatureGates := list "RotateKubeletServerCertificate=true" "CRDSensitiveData=true" -}}
{{- if semverCompare ">=1.31 <1.36" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "TopologyAwareHints=true" -}}
{{- end }}
{{- if semverCompare ">=1.32 <1.34" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "DynamicResourceAllocation=true" -}}
{{- end }}
{{- if semverCompare ">=1.34" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "DRADeviceBindingConditions=true" -}}
  {{- $baseFeatureGates = append $baseFeatureGates "DRAConsumableCapacity=true" -}}
  {{- $baseFeatureGates = append $baseFeatureGates "DRAExtendedResource=true" -}}
{{- end }}
{{- if semverCompare ">=1.33" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "DRAPartitionableDevices=true" -}}
{{- end }}
{{- if semverCompare ">=1.32 <1.33" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "DRAResourceClaimDeviceStatus=true" -}}
{{- end }}
{{- if semverCompare "<=1.32" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "InPlacePodVerticalScaling=true" -}}
{{- end }}
{{- if semverCompare "<=1.31" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "AnonymousAuthConfigurableEndpoints=true" -}}
{{- end }}
{{- $apiserverFeatureGates := $baseFeatureGates -}}
{{- if hasKey . "allowedFeatureGates" -}}
  {{- range .allowedFeatureGates.apiserver -}}
    {{- $apiserverFeatureGates = append $apiserverFeatureGates (printf "%s=true" .) -}}
  {{- end -}}
{{- end -}}
{{- $apiserverFeatureGatesStr := $apiserverFeatureGates | uniq | join "," -}}
{{- $runtimeConfigList := list "admissionregistration.k8s.io/v1beta1=true" "admissionregistration.k8s.io/v1alpha1=true" -}}
{{- if semverCompare ">=1.32 <1.34" .clusterConfiguration.kubernetesVersion }}
  {{- $runtimeConfigList = append $runtimeConfigList "resource.k8s.io/v1beta1=true" -}}
{{- end }}
{{- $runtimeConfig := join "," $runtimeConfigList -}}
{{- $admissionPlugins := list "NodeRestriction" "PodNodeSelector" "PodTolerationRestriction" "EventRateLimit" "ExtendedResourceToleration" -}}
{{- if .apiserver.admissionPlugins -}}
  {{- $admissionPlugins = concat $admissionPlugins .apiserver.admissionPlugins | uniq -}}
{{- end -}}
{{- $sa := .apiserver.serviceAccount | default dict -}}
{{- $primaryAud := $sa.issuer -}}
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
{{- $serviceAccountIssuer := $sa.issuer -}}
{{- if not $serviceAccountIssuer -}}
  {{- $serviceAccountIssuer = printf "https://kubernetes.default.svc.%s" .clusterConfiguration.clusterDomain -}}
{{- end -}}
{{- $serviceAccountJWKSURI := printf "%s/openid/v1/jwks" $serviceAccountIssuer -}}
{{- $bindAddress := "0.0.0.0" -}}
{{- if .apiserver.bindToWildcard -}}
  {{- $bindAddress = "0.0.0.0" -}}
{{- else if .nodeIP -}}
  {{- $bindAddress = .nodeIP -}}
{{- end -}}
{{- $etcdServers := "https://127.0.0.1:2379" -}}
{{- if .apiserver.etcdServers -}}
  {{- $etcdServers = printf "https://127.0.0.1:2379,%s" (.apiserver.etcdServers | join ",") -}}
{{- end -}}
{{- $resourcesRequests := dict -}}
{{- if and .settings .settings.resourcesRequests -}}
  {{- $resourcesRequests = .settings.resourcesRequests -}}
{{- end -}}
{{- $millicpu := $resourcesRequests.milliCPU | default 512 -}}
{{- $memory := $resourcesRequests.memoryBytes | default 536870912 }}
{{- /* kube-apiserver */ -}}
apiVersion: v1
kind: Pod
metadata:
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ .clusterConfiguration.kubernetesVersion | quote }}
    control-plane-manager.deckhouse.io/kube-apiserver.advertise-address.endpoint: {{ .nodeIP | quote }}
  labels:
    component: kube-apiserver
    tier: control-plane
  name: kube-apiserver
  namespace: kube-system
spec:
{{- if eq .runType "ClusterBootstrap" }}
{{- /*
  # Bootstrap-only: cpm rewrites this manifest once during initial install and
  # the default 30s grace would burn most of a minute waiting for the old
  # kube-apiserver to drain (no real in-flight work on a fresh master anyway).
  # Runtime restarts keep the static-pod default 30s so production drains
  # in-flight requests gracefully.
*/}}
  terminationGracePeriodSeconds: 1
{{- end }}
{{- if .apiserver.oidcIssuerAddress }}
  {{- if .apiserver.oidcIssuerURL }}
  hostAliases:
  - ip: {{ .apiserver.oidcIssuerAddress }}
    hostnames:
    - {{ trimSuffix "/" (trimPrefix "https://" .apiserver.oidcIssuerURL) }}
  {{- end }}
{{- end }}
  containers:
  - command:
    - kube-apiserver
    - --api-audiences={{ $audiences | join "," }}
    - --service-account-issuer={{ $serviceAccountIssuer }}
    - --service-account-jwks-uri={{ $serviceAccountJWKSURI }}
    - --service-account-key-file=/etc/kubernetes/pki/sa.pub
    - --service-account-signing-key-file=/etc/kubernetes/pki/sa.key
    - --tls-cert-file=/etc/kubernetes/pki/apiserver.crt
    - --tls-private-key-file=/etc/kubernetes/pki/apiserver.key
    - --client-ca-file=/etc/kubernetes/pki/ca.crt
    - --secure-port=6443
    - --etcd-cafile=/etc/kubernetes/pki/etcd/ca.crt
    - --etcd-certfile=/etc/kubernetes/pki/apiserver-etcd-client.crt
    - --etcd-keyfile=/etc/kubernetes/pki/apiserver-etcd-client.key
{{- if not .apiserver.webhookURL }}
    - --authorization-mode=Node,RBAC
{{- end }}
    - --proxy-client-cert-file=/etc/kubernetes/pki/front-proxy-client.crt
    - --proxy-client-key-file=/etc/kubernetes/pki/front-proxy-client.key
    - --requestheader-allowed-names=front-proxy-client
    - --requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt
    - --requestheader-extra-headers-prefix=X-Remote-Extra-
    - --requestheader-group-headers=X-Remote-Group
    - --requestheader-username-headers=X-Remote-User
    - --service-cluster-ip-range={{ .clusterConfiguration.serviceSubnetCIDR }}
{{- if hasKey . "nodeIP" }}
    - --advertise-address={{ .nodeIP }}
{{- end }}
    - --enable-bootstrap-token-auth=true
    - --allow-privileged=true
{{- if ne .runType "ClusterBootstrap" }}
    - --enable-admission-plugins={{ $admissionPlugins | sortAlpha | join "," }}
  {{- if .apiserver.disableAdmissionPlugins }}
    - --disable-admission-plugins={{ .apiserver.disableAdmissionPlugins }}
  {{- end }}
    - --admission-control-config-file=/etc/kubernetes/deckhouse/extra-files/admission-control-config.yaml
    - --kubelet-certificate-authority=/etc/kubernetes/pki/ca.crt
{{- end }}
    - --kubelet-client-certificate=/etc/kubernetes/pki/apiserver-kubelet-client.crt
    - --kubelet-client-key=/etc/kubernetes/pki/apiserver-kubelet-client.key
    - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
{{- if .apiserver.auditPolicy }}
    - --audit-policy-file=/etc/kubernetes/deckhouse/extra-files/audit-policy.yaml
    - --audit-log-format=json
  {{- if eq .apiserver.auditLog.output "File" }}
    - --audit-log-path=/var/log/kube-audit/audit.log
    - --audit-log-truncate-enabled=true
    - --audit-log-maxage=30
    - --audit-log-maxsize=100
    - --audit-log-maxbackup=10
  {{- else }}
    - --audit-log-path=-
  {{- end }}
{{- end }}
    - --bind-address={{ $bindAddress }}
{{- if hasKey . "arguments" }}
  {{- if hasKey .arguments "defaultUnreachableTolerationSeconds" }}
    - --default-unreachable-toleration-seconds={{ .arguments.defaultUnreachableTolerationSeconds }}
  {{- end }}
  {{- if hasKey .arguments "podEvictionTimeout" }}
    - --default-not-ready-toleration-seconds={{ .arguments.podEvictionTimeout }}
  {{- end }}
{{- end }}
    - --etcd-servers={{ $etcdServers }}
    - --feature-gates={{ $apiserverFeatureGatesStr }}
    - --runtime-config={{ $runtimeConfig }}
{{- if .apiserver.webhookURL }}
    - --authorization-config=/etc/kubernetes/deckhouse/extra-files/authorization-config.yaml
{{- end }}
{{- if .apiserver.authnWebhookURL }}
    - --authentication-token-webhook-config-file=/etc/kubernetes/deckhouse/extra-files/authn-webhook-config.yaml
{{- end }}
{{- if .apiserver.authnWebhookCacheTTL }}
    - --authentication-token-webhook-cache-ttl={{ .apiserver.authnWebhookCacheTTL }}
{{- end }}
{{- if .apiserver.auditWebhookURL }}
    - --audit-webhook-config-file=/etc/kubernetes/deckhouse/extra-files/audit-webhook-config.yaml
{{- end }}
{{- if or (.apiserver.secretEncryptionKey) (.apiserver.signature) }}
    - --encryption-provider-config=/etc/kubernetes/deckhouse/extra-files/secret-encryption-config.yaml
    - --encryption-provider-config-automatic-reload=true
{{- end }}
    - --profiling=false
    - --request-timeout=60s
    - --tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
    - --authentication-config=/etc/kubernetes/deckhouse/extra-files/authentication-config.yaml
{{- if .apiserver.serviceAccount }}
  {{- if .apiserver.serviceAccount.additionalAPIIssuers }}
  {{- $defaultIssuer := printf "https://kubernetes.default.svc.%s" .clusterConfiguration.clusterDomain }}
  {{- $issuerToRemove := default $defaultIssuer .apiserver.serviceAccount.issuer }}
  {{- $uniqueIssuers := .apiserver.serviceAccount.additionalAPIIssuers | uniq }}
    {{- if not (and (eq (len $uniqueIssuers) 1) (eq (index $uniqueIssuers 0) $issuerToRemove)) }}
      {{- range $uniqueIssuers }}
        {{- if ne . $issuerToRemove }}
    - --service-account-issuer={{ . }}
        {{- end }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
    - --watch-cache-sizes=manifestcheckpointcontentchunks.state-snapshotter.deckhouse.io#0
    env:
    - name: GOGC
      value: "50"
{{- if (.images).controlPlaneManager }}  
  {{- $imageWithVersion := printf "kubeApiserver%s" (.clusterConfiguration.kubernetesVersion | replace "." "") }}
  {{- if hasKey .images.controlPlaneManager $imageWithVersion }}
    image: {{ printf "%s%s@%s" .registry.address .registry.path (index .images.controlPlaneManager $imageWithVersion) }}
    imagePullPolicy: IfNotPresent
  {{- end }}
{{- end }}
    livenessProbe:
      failureThreshold: 8
      httpGet:
{{- if hasKey . "nodeIP" }}
        host: {{ .nodeIP }}
{{- end }}
        path: /livez
        port: 6443
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    name: kube-apiserver
    readinessProbe:
      failureThreshold: 3
      httpGet:
{{- if hasKey . "nodeIP" }}
        host: {{ .nodeIP }}
{{- end }}
        path: /healthz
        port: 6443
        scheme: HTTPS
      periodSeconds: 1
      timeoutSeconds: 15
    resources:
      requests:
        cpu: "{{ div (mul $millicpu 33) 100 }}m"
        memory: "{{ div (mul $memory 33) 100 }}"
    securityContext:
      capabilities:
        drop:
        - ALL
      readOnlyRootFilesystem: true
      runAsGroup: 0
      runAsNonRoot: false
      runAsUser: 0
      seccompProfile:
        type: RuntimeDefault
    startupProbe:
      failureThreshold: 24
      httpGet:
{{- if hasKey . "nodeIP" }}
        host: {{ .nodeIP }}
{{- end }}
        path: /livez
        port: 6443
        scheme: HTTPS
{{- if eq .runType "ClusterBootstrap" }}
{{- /*
      # Bootstrap-only: poke /livez aggressively so kubelet flips the pod to
      # "started" as soon as apiserver responds (sub-second on fresh master).
      # Cluster-runtime keeps the conservative 10s+24 settings — restarts in
      # prod tolerate slower probes safely.
*/}}
      initialDelaySeconds: 1
      periodSeconds: 1
      timeoutSeconds: 5
{{- else }}
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
{{- end }}
    volumeMounts:
    - mountPath: /etc/kubernetes/deckhouse/extra-files
      name: deckhouse-extra-files
      readOnly: true
    - mountPath: /etc/pki
      name: etc-pki
      readOnly: true
    - mountPath: /usr/share/ca-certificates
      name: usr-share-ca-certificates
      readOnly: true
    - mountPath: /usr/local/share/ca-certificates
      name: usr-local-share-ca-certificates
      readOnly: true
    - mountPath: /etc/ssl/certs
      name: ca-certs
      readOnly: true
    - mountPath: /etc/kubernetes/pki
      name: k8s-certs
      readOnly: true
{{- if .apiserver.auditPolicy }}
  {{- if eq .apiserver.auditLog.output "File" }}
    - mountPath: /var/log/kube-audit
      name: kube-audit-log
      readOnly: false
  {{- end }}
{{- end }}
  dnsPolicy: ClusterFirstWithHostNet
  hostNetwork: true
  priority: 2000001000
  priorityClassName: system-node-critical
  securityContext:
    seccompProfile:
      type: RuntimeDefault
  volumes:
  - hostPath:
      path: /etc/kubernetes/deckhouse/extra-files
      type: DirectoryOrCreate
    name: deckhouse-extra-files
  - hostPath:
      path: /etc/pki
      type: DirectoryOrCreate
    name: etc-pki
  - hostPath:
      path: /etc/ssl/certs
      type: DirectoryOrCreate
    name: ca-certs
  - hostPath:
      path: /etc/kubernetes/pki
      type: DirectoryOrCreate
    name: k8s-certs
  - hostPath:
      path: /usr/share/ca-certificates
      type: DirectoryOrCreate
    name: usr-share-ca-certificates
  - hostPath:
      path: /usr/local/share/ca-certificates
      type: DirectoryOrCreate
    name: usr-local-share-ca-certificates
{{- if .apiserver.auditPolicy }}
  {{- if eq .apiserver.auditLog.output "File" }}
  - hostPath:
      path: "{{ .apiserver.auditLog.path }}"
      type: DirectoryOrCreate
    name: kube-audit-log
  {{- end }}
{{- end }}
