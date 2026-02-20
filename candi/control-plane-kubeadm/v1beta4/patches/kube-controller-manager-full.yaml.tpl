
    {{- $millicpu := $.resourcesRequestsMilliCpuControlPlane | default 512 -}}
    {{- $memory := $.resourcesRequestsMemoryControlPlane | default 536870912 }}
    {{- $nodeMonitorPeriod := .arguments.nodeMonitorPeriod | default "5" -}}
    {{- $nodeMonitorGracePeriod := .arguments.nodeMonitorGracePeriod | default "40" -}}
    {{- $nodesCount := .nodesCount | default 0 | int }}
    {{- $gcThresholdCount := 1000 }}
    {{- if lt $nodesCount 100 }}
        {{- $gcThresholdCount = 1000 }}
    {{- else if lt $nodesCount 300 }}
        {{- $gcThresholdCount = 3000 }}
    {{- else }}
        {{- $gcThresholdCount = 6000 }}
    {{- end }}

    {{- $baseFeatureGates := list "TopologyAwareHints=true" "RotateKubeletServerCertificate=true" -}}
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
        - --leader-elect=true
        - --requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt
        - --root-ca-file=/etc/kubernetes/pki/ca.crt
        - --service-account-private-key-file=/etc/kubernetes/pki/sa.key
        - --service-cluster-ip-range={{ .clusterConfiguration.serviceSubnetCIDR }}
        - --use-service-account-credentials=true
        - --profiling=false
        - --terminated-pod-gc-threshold={{ $gcThresholdCount }}
        - --feature-gates={{ $controllerManagerFeatureGatesStr | quote }}
        - --node-cidr-mask-size={{ .clusterConfiguration.podSubnetNodeCIDRPrefix | quote }}
        - --bind-address=127.0.0.1
        - --node-monitor-period={{ $nodeMonitorPeriod }}s
        - --node-monitor-grace-period={{ $nodeMonitorGracePeriod }}s
        {{- if eq .clusterConfiguration.clusterType "Cloud" }}
        - -- cloud-provider external
        {{- end }}
      env:
        - name: GOGC
          value: "50"
     {{- if hasKey $ "images" }}
      {{- if hasKey $.images "controlPlaneManager" }}
       {{- $imageWithVersion := printf "kubeControllerManager%s" ($.clusterConfiguration.kubernetesVersion | replace "." "") }}
         {{- if hasKey $.images.controlPlaneManager $imageWithVersion }}
      image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager $imageWithVersion) }}
        {{- end }}
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
          cpu: "{{ div (mul $millicpu 20) 100 }}m"
          memory: "{{ div (mul $memory 20) 100 }}"
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

