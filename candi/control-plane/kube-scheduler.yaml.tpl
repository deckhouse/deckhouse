{{- $baseFeatureGates := list "RotateKubeletServerCertificate=true" -}}
{{- if semverCompare ">=1.31 <1.36" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "TopologyAwareHints=true" -}}
{{- end }}
{{- /* DynamicResourceAllocation: GA default=true since 1.34, explicitly enable for 1.32-1.33 */ -}}
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
{{- if semverCompare "<=1.32" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "InPlacePodVerticalScaling=true" -}}
{{- end }}
{{- if semverCompare "<=1.31" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "AnonymousAuthConfigurableEndpoints=true" -}}
{{- end }}
{{- $schedulerFeatureGates := $baseFeatureGates -}}
{{- if hasKey . "allowedFeatureGates" -}}
  {{- range .allowedFeatureGates.kubeScheduler -}}
    {{- $schedulerFeatureGates = append $schedulerFeatureGates (printf "%s=true" .) -}}
  {{- end -}}
{{- end -}}
{{- $schedulerFeatureGatesStr := $schedulerFeatureGates | uniq | join "," -}}
{{- $resourcesRequests := dict -}}
{{- if and $.settings $.settings.resourcesRequests -}}
  {{- $resourcesRequests = $.settings.resourcesRequests -}}
{{- end -}}
{{- $nodesCount := .nodesCount | default 0 | int -}}
{{- $maxMilliCPU := $resourcesRequests.maxMilliCPU | default 0 | int -}}
{{- $maxMemoryBytes := $resourcesRequests.maxMemoryBytes | default 0 | int -}}
{{- /*
  Resource requests for the kube-scheduler static pod (component share: 10%).

  Manual override (controlPlaneManager.resourcesRequests) arrives as a single
  pool and is split by the historical component share (CPU and memory
  independently). Otherwise requests are sized per-component in discrete tiers
  by the cluster node count — stepped, not linear, so the static pod stays
  stable within a tier and only changes at rare tier boundaries. The auto value
  is clamped to its share of the node safety cap ($maxMilliCPU / $maxMemoryBytes)
  computed by the hook, so it never crowds out other pods on an undersized master.
*/ -}}
{{- $millicpu := 0 -}}
{{- $memory := 0 -}}
{{- if $resourcesRequests.milliCPU -}}
  {{- $millicpu = div (mul $resourcesRequests.milliCPU 10) 100 -}}
{{- else -}}
  {{- if lt $nodesCount 25 -}}{{- $millicpu = 30 -}}
  {{- else if lt $nodesCount 100 -}}{{- $millicpu = 40 -}}
  {{- else if lt $nodesCount 250 -}}{{- $millicpu = 60 -}}
  {{- else if lt $nodesCount 500 -}}{{- $millicpu = 80 -}}
  {{- else -}}{{- $millicpu = 120 -}}
  {{- end -}}
  {{- if $maxMilliCPU -}}{{- $millicpu = min $millicpu (div (mul $maxMilliCPU 10) 100) -}}{{- end -}}
{{- end -}}
{{- if $resourcesRequests.memoryBytes -}}
  {{- $memory = div (mul $resourcesRequests.memoryBytes 10) 100 -}}
{{- else -}}
  {{- if lt $nodesCount 10 -}}{{- $memory = 128 -}}
  {{- else if lt $nodesCount 25 -}}{{- $memory = 256 -}}
  {{- else if lt $nodesCount 100 -}}{{- $memory = 384 -}}
  {{- else -}}{{- $memory = 512 -}}
  {{- end -}}
  {{- $memory = mul $memory 1048576 -}}
  {{- if $maxMemoryBytes -}}{{- $memory = min $memory (div (mul $maxMemoryBytes 10) 100) -}}{{- end -}}
{{- end }}
apiVersion: v1
kind: Pod
metadata:
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ $.clusterConfiguration.kubernetesVersion | quote }}
  labels:
    component: kube-scheduler
    tier: control-plane
  name: kube-scheduler
  namespace: kube-system
spec:
  containers:
  - command:
    - kube-scheduler
    - --authentication-kubeconfig=/etc/kubernetes/scheduler.conf
    - --authorization-kubeconfig=/etc/kubernetes/scheduler.conf
    - --kubeconfig=/etc/kubernetes/scheduler.conf
    - --leader-elect=true
    - --profiling=false
    - --feature-gates={{ $schedulerFeatureGatesStr }}
    - --bind-address=127.0.0.1
{{- if ne .runType "ClusterBootstrap" }}
    - --config=/etc/kubernetes/deckhouse/extra-files/scheduler-config.yaml
{{- end }}
    env:
    - name: GOGC
      value: "50"
{{- if (.images).controlPlaneManager }}  
{{- $imageWithVersion := printf "kubeScheduler%s" ($.clusterConfiguration.kubernetesVersion | replace "." "") }}
  {{- if hasKey $.images.controlPlaneManager $imageWithVersion }}
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager $imageWithVersion) }}
  {{- end }}
{{- end }}
    imagePullPolicy: IfNotPresent
    livenessProbe:
      failureThreshold: 8
      httpGet:
        host: 127.0.0.1
        path: /livez
        port: 10259
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    name: kube-scheduler
    readinessProbe:
      failureThreshold: 3
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10259
        scheme: HTTPS
      periodSeconds: 1
      timeoutSeconds: 15
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
        path: /livez
        port: 10259
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    volumeMounts:
    - mountPath: /etc/kubernetes/deckhouse/extra-files
      name: deckhouse-extra-files
      readOnly: true
    - mountPath: /etc/kubernetes/scheduler.conf
      name: kubeconfig
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
      path: /etc/kubernetes/deckhouse/extra-files
      type: DirectoryOrCreate
    name: deckhouse-extra-files
  - hostPath:
      path: /etc/kubernetes/scheduler.conf
      type: FileOrCreate
    name: kubeconfig
