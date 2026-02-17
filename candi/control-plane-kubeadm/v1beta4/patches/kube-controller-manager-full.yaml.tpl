{{- $baseFeatureGates := list "TopologyAwareHints=true" "RotateKubeletServerCertificate=true" -}}
{{- if semverCompare ">=1.32 <1.34" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "DynamicResourceAllocation=true" -}}
{{- end }}
{{- if semverCompare "<=1.32" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "InPlacePodVerticalScaling=true" -}}
{{- end }}
{{- $controllerManagerFeatureGates := $baseFeatureGates -}}
{{- if hasKey . "allowedFeatureGates" -}}
  {{- range .allowedFeatureGates.kubeControllerManager -}}
    {{- $controllerManagerFeatureGates = append $controllerManagerFeatureGates (printf "%s=true" .) -}}
  {{- end -}}
{{- end -}}
{{- $controllerManagerFeatureGatesStr := $controllerManagerFeatureGates | uniq | join "," -}}
{{- $nodesCount := .nodesCount | default 0 | int }}
{{- $gcThresholdCount := 1000 }}
{{- if lt $nodesCount 100 }}
  {{- $gcThresholdCount = 1000 }}
{{- else if lt $nodesCount 300 }}
  {{- $gcThresholdCount = 3000 }}
{{- else }}
  {{- $gcThresholdCount = 6000 }}
{{- end }}
{{- $millicpu := .resourcesRequestsMilliCpuControlPlane | default 512 -}}
{{- $memory := .resourcesRequestsMemoryControlPlane | default 536870912 }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ .clusterConfiguration.kubernetesVersion | quote }}
spec:
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  volumes:
  - name: kubeconfig
    hostPath:
      path: /etc/kubernetes/controller-manager.conf
      type: FileOrCreate
  - name: k8s-certs
    hostPath:
      path: /etc/kubernetes/pki
      type: DirectoryOrCreate
  - name: deckhouse-extra-files
    hostPath:
      path: /etc/kubernetes/deckhouse/extra-files
      type: DirectoryOrCreate
  containers:
  - name: kube-controller-manager
{{- if hasKey . "images" }}
{{- if hasKey .images "controlPlaneManager" }}
{{- $imageWithVersion := printf "kubeControllerManager%s" (.clusterConfiguration.kubernetesVersion | replace "." "") }}
{{- if hasKey .images.controlPlaneManager $imageWithVersion }}
    image: {{ printf "%s%s@%s" .registry.address .registry.path (index .images.controlPlaneManager $imageWithVersion) }}
{{- end }}
{{- end }}
{{- end }}
    command:
    - kube-controller-manager
    args:
    - --bind-address=127.0.0.1
    - --leader-elect=true
    - --kubeconfig=/etc/kubernetes/controller-manager.conf
    - --authentication-kubeconfig=/etc/kubernetes/controller-manager.conf
    - --authorization-kubeconfig=/etc/kubernetes/controller-manager.conf
    - --client-ca-file=/etc/kubernetes/pki/ca.crt
    - --requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt
    - --root-ca-file=/etc/kubernetes/pki/ca.crt
    - --service-account-private-key-file=/etc/kubernetes/pki/sa.key
    - --cluster-signing-cert-file=/etc/kubernetes/pki/ca.crt
    - --cluster-signing-key-file=/etc/kubernetes/pki/ca.key
    - --use-service-account-credentials=true
    - --controllers=*,bootstrapsigner,tokencleaner
    - --allocate-node-cidrs=true
    - --cluster-cidr={{ .clusterConfiguration.podSubnetCIDR | quote }}
    - --service-cluster-ip-range={{ .clusterConfiguration.serviceSubnetCIDR | quote }}
    - --profiling=false
    - --terminated-pod-gc-threshold={{ $gcThresholdCount | quote }}
    - --feature-gates={{ $controllerManagerFeatureGatesStr | quote }}
    - --node-cidr-mask-size={{ .clusterConfiguration.podSubnetNodeCIDRPrefix | quote }}
{{- if eq .clusterConfiguration.clusterType "Cloud" }}
    - --cloud-provider=external
{{- end }}
{{- if hasKey . "arguments" }}
{{- if hasKey .arguments "nodeMonitorPeriod" }}
    - --node-monitor-period={{ .arguments.nodeMonitorPeriod }}s
    - --node-monitor-grace-period={{ .arguments.nodeMonitorGracePeriod }}s
{{- end }}
{{- end }}
    volumeMounts:
    - name: kubeconfig
      mountPath: /etc/kubernetes/controller-manager.conf
      readOnly: true
    - name: k8s-certs
      mountPath: /etc/kubernetes/pki
      readOnly: true
    - name: deckhouse-extra-files
      mountPath: /etc/kubernetes/deckhouse/extra-files
      readOnly: true
    resources:
      requests:
        cpu: "{{ div (mul $millicpu 20) 100 }}m"
        memory: "{{ div (mul $memory 20) 100 }}"
    securityContext:
      runAsNonRoot: false
      runAsUser: 0
      runAsGroup: 0
      capabilities:
        drop:
        - ALL
      readOnlyRootFilesystem: true
      seccompProfile:
        type: RuntimeDefault
    env:
    - name: GOGC
      value: "50"
    readinessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10257
        scheme: HTTPS
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10257
        scheme: HTTPS
