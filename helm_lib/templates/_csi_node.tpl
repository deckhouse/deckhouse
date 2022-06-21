{{- /* Usage: {{ include "helm_lib_csi_node_manifests" (list . $config) }} */ -}}
{{- define "helm_lib_csi_node_manifests" }}
  {{- $context := index . 0 }}

  {{- $config := index . 1 }}
  {{- $fullname := $config.fullname | default "csi-node" }}
  {{- $nodeImage := $config.nodeImage | required "$config.nodeImage is required" }}
  {{- $driverFQDN := $config.driverFQDN | required "$config.driverFQDN is required" }}
  {{- $serviceAccount := $config.serviceAccount | default "" }}
  {{- $additionalNodeEnvs := $config.additionalNodeEnvs }}
  {{- $additionalNodeArgs := $config.additionalNodeArgs }}
  {{- $additionalNodeVolumes := $config.additionalNodeVolumes }}
  {{- $additionalNodeVolumeMounts := $config.additionalNodeVolumeMounts }}

  {{- $kubernetesSemVer := semver $context.Values.global.discovery.kubernetesVersion }}

  {{- $driverRegistrarImageName := join "" (list "csiNodeDriverRegistrar" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $driverRegistrarImageTag := index $context.Values.global.modulesImages.tags.common $driverRegistrarImageName }}
  {{- $driverRegistrarImage := printf "%s:%s" $context.Values.global.modulesImages.registry $driverRegistrarImageTag }}

  {{- if $driverRegistrarImageTag }}
    {{- if or (include "_helm_lib_cloud_or_hybrid_cluster" $context) ($context.Values.global.enabledModules | has "ceph-csi") }}
      {{- if ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" "csi-node" "workload-resource-policy.deckhouse.io" "every-node")) | nindent 2 }}
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: DaemonSet
    name: {{ $fullname }}
  updatePolicy:
    updateMode: "Auto"
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
    spec:
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
                {{- if or (eq $fullname "csi-node-rbd") (eq $fullname "csi-node-cephfs") }}
                - Static
                {{- end }}
      imagePullSecrets:
      - name: deckhouse-registry
      {{- include "helm_lib_priority_class" (tuple $context "system-node-critical") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple $context "any-node-with-no-csi") | nindent 6 }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_root" . | nindent 6 }}
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      containers:
      - name: node-driver-registrar
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all" . | nindent 8 }}
        image: {{ $driverRegistrarImage | quote }}
        args:
        - "--v=5"
        - "--csi-address=$(CSI_ENDPOINT)"
        - "--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)"
        env:
        - name: CSI_ENDPOINT
          value: "/csi/csi.sock"
        - name: DRIVER_REG_SOCK_PATH
          value: "/var/lib/kubelet/csi-plugins/{{ $driverFQDN }}/csi.sock"
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        volumeMounts:
        - name: plugin-dir
          mountPath: /csi
        - name: registration-dir
          mountPath: /registration
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
      - name: node
        securityContext:
          privileged: true
        image: {{ $nodeImage }}
        args:
      {{- if $additionalNodeArgs }}
        {{- $additionalNodeArgs | toYaml | nindent 8 }}
      {{- end }}
      {{- if $additionalNodeEnvs }}
        env:
        {{- $additionalNodeEnvs | toYaml | nindent 8 }}
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
      serviceAccount: {{ $serviceAccount | quote }}
      serviceAccountName: {{ $serviceAccount | quote }}
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
