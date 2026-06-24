{{- $resourcesRequests := dict -}}
{{- if and $.settings $.settings.resourcesRequests -}}
  {{- $resourcesRequests = $.settings.resourcesRequests -}}
{{- end -}}
{{- $nodesCount := .nodesCount | default 0 | int }}
{{- /*
  Resource requests for the kube-controller-manager static pod.
  Manual override (controlPlaneManager.resourcesRequests) arrives as a single
  pool via settings.resourcesRequests and keeps the historical component share
  (20%). Otherwise requests are sized per-component: a fixed floor + linear
  growth by cluster node count, capped.
*/ -}}
{{- $millicpu := 0 -}}
{{- if $resourcesRequests.milliCPU -}}
  {{- $millicpu = div (mul $resourcesRequests.milliCPU 20) 100 -}}
{{- else -}}
  {{- $millicpu = max 50 (min (add 50 (div (mul $nodesCount 4) 10)) 300) -}}
{{- end -}}
{{- $memory := 0 -}}
{{- if $resourcesRequests.memoryBytes -}}
  {{- $memory = div (mul $resourcesRequests.memoryBytes 20) 100 -}}
{{- else -}}
  {{- $memory = mul (max 256 (min (add 256 (mul 4 $nodesCount)) 1536)) 1048576 -}}
{{- end }}
{{- $gcThresholdCount := 1000 }}
{{- if lt $nodesCount 100 }}
    {{- $gcThresholdCount = 1000 }}
{{- else if lt $nodesCount 300 }}
    {{- $gcThresholdCount = 3000 }}
{{- else }}
    {{- $gcThresholdCount = 6000 }}
{{- end }}
{{- $baseFeatureGates := list "RotateKubeletServerCertificate=true" -}}
{{- if semverCompare ">=1.31 <1.36" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "TopologyAwareHints=true" -}}
{{- end }}
{{- /* DynamicResourceAllocation: GA default=true since 1.34, explicitly enable for 1.32-1.33 */ -}}
{{- if semverCompare ">=1.32 <1.34" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "DynamicResourceAllocation=true" -}}
{{- end }}
{{- if semverCompare "<=1.32" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "InPlacePodVerticalScaling=true" -}}
{{- end }}
{{- if semverCompare "<=1.31" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "AnonymousAuthConfigurableEndpoints=true" -}}
{{- end }}
{{- $controllerManagerFeatureGates := $baseFeatureGates -}}
{{- if hasKey . "allowedFeatureGates" -}}
  {{- range .allowedFeatureGates.kubeControllerManager -}}
    {{- $controllerManagerFeatureGates = append $controllerManagerFeatureGates (printf "%s=true" .) -}}
  {{- end -}}
{{- end -}}
{{- $controllerManagerFeatureGatesStr := $controllerManagerFeatureGates | uniq | join "," -}}
apiVersion: v1
kind: Pod
metadata:
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ $.clusterConfiguration.kubernetesVersion | quote }}
  labels:
    component: kube-controller-manager
    tier: control-plane
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
  - command:
    - kube-controller-manager
    - --allocate-node-cidrs=true
    - --authentication-kubeconfig=/etc/kubernetes/controller-manager.conf
    - --authorization-kubeconfig=/etc/kubernetes/controller-manager.conf
    - --client-ca-file=/etc/kubernetes/pki/ca.crt
    - --cluster-cidr={{ .clusterConfiguration.podSubnetCIDR }}
    - --cluster-name=kubernetes
    - --cluster-signing-cert-file=/etc/kubernetes/pki/ca.crt
    - --cluster-signing-key-file=/etc/kubernetes/pki/ca.key
    - --controllers=*,bootstrapsigner,tokencleaner
    - --kubeconfig=/etc/kubernetes/controller-manager.conf
    - --kube-api-qps=-1
    - --leader-elect=true
    - --requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt
    - --root-ca-file=/etc/kubernetes/pki/ca.crt
    - --service-account-private-key-file=/etc/kubernetes/pki/sa.key
    - --service-cluster-ip-range={{ .clusterConfiguration.serviceSubnetCIDR }}
    - --use-service-account-credentials=true
    - --profiling=false
    - --terminated-pod-gc-threshold={{ $gcThresholdCount }}
    - --feature-gates={{ $controllerManagerFeatureGatesStr }}
    - --node-cidr-mask-size={{ .clusterConfiguration.podSubnetNodeCIDRPrefix }}
    - --bind-address=127.0.0.1
    {{- if hasKey . "arguments" }}
      {{- if hasKey .arguments "nodeMonitorPeriod" }}
    - --node-monitor-period={{ .arguments.nodeMonitorPeriod }}s
    - --node-monitor-grace-period={{ .arguments.nodeMonitorGracePeriod }}s
      {{- end }}
    {{- end }}
    {{- if eq .clusterConfiguration.clusterType "Cloud" }}
    - --cloud-provider=external
    {{- end }}
    env:
    - name: GOGC
      value: "50"
    {{- if (.images).controlPlaneManager }}  
      {{- $imageWithVersion := printf "kubeControllerManager%s" ($.clusterConfiguration.kubernetesVersion | replace "." "") }}
        {{- if hasKey $.images.controlPlaneManager $imageWithVersion }}
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager $imageWithVersion) }}
      {{- end }}
    {{- end }}
    imagePullPolicy: IfNotPresent
    livenessProbe:
      failureThreshold: 8
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10257
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    name: kube-controller-manager
    readinessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10257
        scheme: HTTPS
    resources:
      requests:
        cpu: "{{ $millicpu }}m"
        memory: "{{ $memory }}"
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
        host: 127.0.0.1
        path: /healthz
        port: 10257
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    volumeMounts:
    - mountPath: /etc/ssl/certs
      name: ca-certs
      readOnly: true
    - mountPath: /etc/kubernetes/deckhouse/extra-files
      name: deckhouse-extra-files
      readOnly: true
    - mountPath: /usr/libexec/kubernetes/kubelet-plugins/volume/exec
      name: flexvolume-dir
    - mountPath: /etc/kubernetes/pki
      name: k8s-certs
      readOnly: true
    - mountPath: /etc/kubernetes/controller-manager.conf
      name: kubeconfig
      readOnly: true
    - mountPath: /usr/share/ca-certificates
      name: usr-share-ca-certificates
      readOnly: true
  dnsPolicy: ClusterFirstWithHostNet
  hostNetwork: true
  priority: 2000001000
  priorityClassName: system-node-critical
  securityContext:
    seccompProfile:
      type: RuntimeDefault
  volumes:
  - hostPath:
      path: /etc/ssl/certs
      type: DirectoryOrCreate
    name: ca-certs
  - hostPath:
      path: /etc/kubernetes/deckhouse/extra-files
      type: DirectoryOrCreate
    name: deckhouse-extra-files
  - hostPath:
      path: /usr/libexec/kubernetes/kubelet-plugins/volume/exec
      type: DirectoryOrCreate
    name: flexvolume-dir
  - hostPath:
      path: /etc/kubernetes/pki
      type: DirectoryOrCreate
    name: k8s-certs
  - hostPath:
      path: /etc/kubernetes/controller-manager.conf
      type: FileOrCreate
    name: kubeconfig
  - hostPath:
      path: /usr/share/ca-certificates
      type: DirectoryOrCreate
    name: usr-share-ca-certificates
