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
{{- $schedulerFeatureGates := $baseFeatureGates -}}
{{- if hasKey . "allowedFeatureGates" -}}
  {{- range .allowedFeatureGates.kubeScheduler -}}
    {{- $schedulerFeatureGates = append $schedulerFeatureGates (printf "%s=true" .) -}}
  {{- end -}}
{{- end -}}
{{- $schedulerFeatureGatesStr := $schedulerFeatureGates | uniq | join "," -}}
{{- $millicpu := $.resourcesRequestsMilliCpuControlPlane | default 512 -}}
{{- $memory := $.resourcesRequestsMemoryControlPlane | default 536870912 }}
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
        - --feature-gates={{ $schedulerFeatureGatesStr | quote }}
        - --bind-address=127.0.0.1
        {{- if ne .runType "ClusterBootstrap" }}
        - --config=/etc/kubernetes/deckhouse/extra-files/scheduler-config.yaml
        {{- end }}
      env:
        - name: GOGC
          value: "50"
      {{- if hasKey $ "images" }}
        {{- if hasKey $.images "controlPlaneManager" }}
          {{- $imageWithVersion := printf "kubeScheduler%s" ($.clusterConfiguration.kubernetesVersion | replace "." "") }}
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
          cpu: "{{ div (mul $millicpu 10) 100 }}m"
          memory: "{{ div (mul $memory 10) 100 }}"
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
