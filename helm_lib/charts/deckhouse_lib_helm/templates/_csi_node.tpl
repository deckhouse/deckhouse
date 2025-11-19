{{- define "node_driver_registrar_resources" }}
cpu: 12m
memory: 25Mi
{{- end }}

{{- define "node_resources" }}
cpu: 12m
memory: 25Mi
{{- end }}

{{- /* Usage: {{ include "helm_lib_csi_node_manifests" (list . $config) }} */ -}}
{{- define "helm_lib_csi_node_manifests" }}
  {{- $context := index . 0 }}

  {{- $config := index . 1 }}
  {{- $fullname := $config.fullname | default "csi-node" }}
  {{- $nodeImage := $config.nodeImage | required "$config.nodeImage is required" }}
  {{- $driverFQDN := $config.driverFQDN | required "$config.driverFQDN is required" }}
  {{- $serviceAccount := $config.serviceAccount | default "" }}
  {{- $additionalNodeVPA := $config.additionalNodeVPA }}
  {{- $additionalNodeEnvs := $config.additionalNodeEnvs }}
  {{- $additionalNodeArgs := $config.additionalNodeArgs }}
  {{- $additionalNodeVolumes := $config.additionalNodeVolumes }}
  {{- $additionalNodeVolumeMounts := $config.additionalNodeVolumeMounts }}
  {{- $additionalNodeLivenessProbesCmd := $config.additionalNodeLivenessProbesCmd }}
  {{- $livenessProbePort := $config.livenessProbePort }}
  {{- $additionalNodeSelectorTerms := $config.additionalNodeSelectorTerms }}
  {{- $customNodeSelector := $config.customNodeSelector }}
  {{- $forceCsiNodeAndStaticNodesDepoloy := $config.forceCsiNodeAndStaticNodesDepoloy | default false }}
  {{- $setSysAdminCapability := $config.setSysAdminCapability | default false }}
  {{- $additionalContainers := $config.additionalContainers }} 
  {{- $initContainers := $config.initContainers }}
  {{- $additionalPullSecrets := $config.additionalPullSecrets }}
  {{- $csiNodeLifecycle := $config.csiNodeLifecycle | default false }}
  {{- $csiNodeDriverRegistrarLifecycle := $config.csiNodeDriverRegistrarLifecycle | default false }}
  {{- $additionalCsiNodePodAnnotations := $config.additionalCsiNodePodAnnotations | default false }}
  {{- $csiNodeHostNetwork := $config.csiNodeHostNetwork | default "true" }}
  {{- $csiNodeHostPID := $config.csiNodeHostPID | default "false" }}
  {{- $kubernetesSemVer := semver $context.Values.global.discovery.kubernetesVersion }}
  {{- $driverRegistrarImageName := join "" (list "csiNodeDriverRegistrar" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $driverRegistrarImage := include "helm_lib_module_common_image_no_fail" (list $context $driverRegistrarImageName) }}
  {{- if $driverRegistrarImage }}
    {{- if or $forceCsiNodeAndStaticNodesDepoloy (include "_helm_lib_cloud_or_hybrid_cluster" $context) }}
      {{- if ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" "csi-node")) | nindent 2 }}
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: DaemonSet
    name: {{ $fullname }}
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    {{- if $additionalNodeVPA }}
    {{- $additionalNodeVPA | toYaml | nindent 4 }}
    {{- end }}
    - containerName: "node-driver-registrar"
      minAllowed:
        {{- include "node_driver_registrar_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: 25m
        memory: 50Mi
    - containerName: "node"
      minAllowed:
        {{- include "node_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: 25m
        memory: 50Mi
    {{- end }}
---
kind: DaemonSet
apiVersion: apps/v1
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" "csi-node")) | nindent 2 }}
spec:
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app: {{ $fullname }}
  template:
    metadata:
      labels:
        app: {{ $fullname }}
      {{- if or (hasPrefix "cloud-provider-" $context.Chart.Name) ($additionalCsiNodePodAnnotations) }}
      annotations:
      {{- if hasPrefix "cloud-provider-" $context.Chart.Name }}
        cloud-config-checksum: {{ include (print $context.Template.BasePath "/cloud-controller-manager/secret.yaml") $context | sha256sum }}
      {{- end }}
      {{- if $additionalCsiNodePodAnnotations }}
        {{- $additionalCsiNodePodAnnotations | toYaml | nindent 8 }}
      {{- end }}
      {{- end }}
    spec:
      {{- if $customNodeSelector }}
      nodeSelector:
        {{- $customNodeSelector | toYaml | nindent 8 }}
      {{- else }}
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - operator: In
                key: node.deckhouse.io/type
                values:
                - CloudEphemeral
                - CloudPermanent
                - CloudStatic
                {{- if $forceCsiNodeAndStaticNodesDepoloy }}
                - Static
                {{- end }}
              {{- if $additionalNodeSelectorTerms }}
              {{- $additionalNodeSelectorTerms | toYaml | nindent 14 }}
              {{- end }}
      {{- end }}
      imagePullSecrets:
      - name: deckhouse-registry
      {{- if $additionalPullSecrets }}
      {{- $additionalPullSecrets | toYaml | nindent 6 }}
      {{- end }}
      {{- include "helm_lib_priority_class" (tuple $context "system-node-critical") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple $context "any-node" "with-no-csi") | nindent 6 }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_root" . | nindent 6 }}
      hostNetwork: {{ $csiNodeHostNetwork }}
      hostPID: {{ $csiNodeHostPID }}
      {{- if eq $csiNodeHostNetwork "true" }}
      dnsPolicy: ClusterFirstWithHostNet
      {{- end }}
      containers:
      - name: node-driver-registrar
        {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true "seccompProfile" true "uid" "0" "runAsNonRoot" false) | nindent 8 }}
        image: {{ $driverRegistrarImage | quote }}
        args:
        - "--v=5"
        - "--csi-address=$(CSI_ENDPOINT)"
        - "--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)"
        {{- if $livenessProbePort }}
        - "--http-endpoint=:{{ $livenessProbePort }}"
        {{- end }}
        env:
        - name: CSI_ENDPOINT
          value: "/csi/csi.sock"
        - name: DRIVER_REG_SOCK_PATH
          value: "/var/lib/kubelet/csi-plugins/{{ $driverFQDN }}/csi.sock"
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
      {{- if $csiNodeDriverRegistrarLifecycle }}
        lifecycle:
          {{- $csiNodeDriverRegistrarLifecycle | toYaml | nindent 10 }}
      {{- end }}
      {{- if $additionalNodeLivenessProbesCmd }}
        livenessProbe:
          initialDelaySeconds: 3
          exec:
            command:
        {{- $additionalNodeLivenessProbesCmd | toYaml | nindent 12 }}
      {{- end }}
        volumeMounts:
        - name: plugin-dir
          mountPath: /csi
        - name: registration-dir
          mountPath: /registration
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" 10 | nindent 12 }}
  {{- if not ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
            {{- include "node_driver_registrar_resources" $context | nindent 12 }}
  {{- end }}
      - name: node
        securityContext:
          privileged: true
          readOnlyRootFilesystem: true
          seccompProfile:
            type: RuntimeDefault
        {{- if $setSysAdminCapability }}
          capabilities:
            add:
            - SYS_ADMIN
        {{- end }}
        image: {{ $nodeImage }}
        args:
      {{- if $additionalNodeArgs }}
        {{- $additionalNodeArgs | toYaml | nindent 8 }}
      {{- end }}
      {{- if $additionalNodeEnvs }}
        env:
        {{- $additionalNodeEnvs | toYaml | nindent 8 }}
      {{- end }}
      {{- if $csiNodeLifecycle }}
        lifecycle:
          {{- $csiNodeLifecycle | toYaml | nindent 10 }}
      {{- end }}
      {{- if $livenessProbePort }}
        livenessProbe:
          httpGet:
            path: /healthz
            port: {{ $livenessProbePort }}
          initialDelaySeconds: 5
          timeoutSeconds: 5
      {{- end }}      
        volumeMounts:
        - name: kubelet-dir
          mountPath: /var/lib/kubelet
          mountPropagation: "Bidirectional"
        - name: plugin-dir
          mountPath: /csi
        - name: device-dir
          mountPath: /dev
        {{- if $additionalNodeVolumeMounts }}
          {{- $additionalNodeVolumeMounts | toYaml | nindent 8 }}
        {{- end }}
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
  {{- if not ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
            {{- include "node_resources" $context | nindent 12 }}
  {{- end }}

      {{- if $additionalContainers }}
        {{- $additionalContainers | toYaml | nindent 6 }}
      {{- end }}

  {{- if $initContainers }}
      initContainers:
    {{- range $initContainer := $initContainers }}
      - resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
        {{- $initContainer | toYaml | nindent 8 }}
    {{- end }}
  {{- end }}

      serviceAccount: {{ $serviceAccount | quote }}
      serviceAccountName: {{ $serviceAccount | quote }}
      automountServiceAccountToken: true
      volumes:
      - name: registration-dir
        hostPath:
          path: /var/lib/kubelet/plugins_registry/
          type: Directory
      - name: kubelet-dir
        hostPath:
          path: /var/lib/kubelet
          type: Directory
      - name: plugin-dir
        hostPath:
          path: /var/lib/kubelet/csi-plugins/{{ $driverFQDN }}/
          type: DirectoryOrCreate
      - name: device-dir
        hostPath:
          path: /dev
          type: Directory

      {{- if $additionalNodeVolumes }}
        {{- $additionalNodeVolumes | toYaml | nindent 6 }}
      {{- end }}

    {{- end }}
  {{- end }}
{{- end }}
