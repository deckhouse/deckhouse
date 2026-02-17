{{- $baseFeatureGates := list "TopologyAwareHints=true" "RotateKubeletServerCertificate=true" -}}
{{- if semverCompare ">=1.32 <1.34" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "DynamicResourceAllocation=true" -}}
{{- end }}
{{- if semverCompare "<=1.32" .clusterConfiguration.kubernetesVersion }}
  {{- $baseFeatureGates = append $baseFeatureGates "InPlacePodVerticalScaling=true" -}}
{{- end }}
{{- $schedulerFeatureGates := $baseFeatureGates -}}
{{- if hasKey . "allowedFeatureGates" -}}
  {{- range .allowedFeatureGates.kubeScheduler -}}
    {{- $schedulerFeatureGates = append $schedulerFeatureGates (printf "%s=true" .) -}}
  {{- end -}}
{{- end -}}
{{- $schedulerFeatureGatesStr := $schedulerFeatureGates | uniq | join "," -}}
{{- $millicpu := .resourcesRequestsMilliCpuControlPlane | default 512 -}}
{{- $memory := .resourcesRequestsMemoryControlPlane | default 536870912 }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
  labels:
    component: kube-scheduler
    tier: control-plane
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ .clusterConfiguration.kubernetesVersion | quote }}
spec:
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  priority: 2000001000
  priorityClassName: system-node-critical
  volumes:
  - name: kubeconfig
    hostPath:
      path: /etc/kubernetes/scheduler.conf
      type: FileOrCreate
  - name: deckhouse-extra-files
    hostPath:
      path: /etc/kubernetes/deckhouse/extra-files
      type: DirectoryOrCreate
  containers:
  - name: kube-scheduler
{{- if hasKey . "images" }}
{{- if hasKey .images "controlPlaneManager" }}
{{- $imageWithVersion := printf "kubeScheduler%s" (.clusterConfiguration.kubernetesVersion | replace "." "") }}
{{- if hasKey .images.controlPlaneManager $imageWithVersion }}
    image: {{ printf "%s%s@%s" .registry.address .registry.path (index .images.controlPlaneManager $imageWithVersion) }}
{{- end }}
{{- end }}
{{- end }}
    command:
    - kube-scheduler
    - --bind-address=127.0.0.1
    - --leader-elect=true
    - --kubeconfig=/etc/kubernetes/scheduler.conf
    - --authentication-kubeconfig=/etc/kubernetes/scheduler.conf
    - --authorization-kubeconfig=/etc/kubernetes/scheduler.conf
    - --profiling=false
    - --feature-gates={{ $schedulerFeatureGatesStr | quote }}
{{- if ne .runType "ClusterBootstrap" }}
    - --config=/etc/kubernetes/deckhouse/extra-files/scheduler-config.yaml
{{- end }}
    volumeMounts:
    - name: kubeconfig
      mountPath: /etc/kubernetes/scheduler.conf
      readOnly: true
    - name: deckhouse-extra-files
      mountPath: /etc/kubernetes/deckhouse/extra-files
      readOnly: true
    resources:
      requests:
        cpu: "{{ div (mul $millicpu 10) 100 }}m"
        memory: "{{ div (mul $memory 10) 100 }}"
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
      failureThreshold: 3
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10259
        scheme: HTTPS
      periodSeconds: 1
      timeoutSeconds: 15
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
    startupProbe:
      failureThreshold: 24
      httpGet:
        host: 127.0.0.1
        path: /livez
        port: probe-port
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15