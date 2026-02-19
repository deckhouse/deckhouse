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
  labels:
    component: kube-controller-manager
    tier: control-plane
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ .clusterConfiguration.kubernetesVersion | quote }}
spec:
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  priority: 2000001000
  priorityClassName: system-node-critical
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
  - hostPath:
      path: /usr/local/share/ca-certificates
      type: DirectoryOrCreate
    name: usr-local-share-ca-certificates
  containers:
  - name: kube-controller-manager
{{- if hasKey . "images" }}
{{- if hasKey .images "controlPlaneManager" }}
{{- $imageWithVersion := printf "kubeControllerManager%s" (.clusterConfiguration.kubernetesVersion | replace "." "") }}
{{- if hasKey .images.controlPlaneManager $imageWithVersion }}
    image: {{ printf "%s%s@%s" .registry.address .registry.path (index .images.controlPlaneManager $imageWithVersion) }}
    imagePullPolicy: IfNotPresent
{{- end }}
{{- end }}
{{- end }}
    command:
    - kube-controller-manager
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
    - --cluster-cidr={{ .clusterConfiguration.podSubnetCIDR }}
    - --cluster-name=kubernetes
    - --service-cluster-ip-range={{ .clusterConfiguration.serviceSubnetCIDR }}
    - --profiling=false
    - --terminated-pod-gc-threshold={{ $gcThresholdCount }}
    - --feature-gates={{ $controllerManagerFeatureGatesStr }}
    - --node-cidr-mask-size={{ .clusterConfiguration.podSubnetNodeCIDRPrefix }}
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
    - mountPath: /usr/local/share/ca-certificates
      name: usr-local-share-ca-certificates
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
    ports:
    - containerPort: 10257
      name: probe-port
      protocol: TCP
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
    startupProbe:
      failureThreshold: 24
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: probe-port
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15