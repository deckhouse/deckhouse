{{- /* Usage: {{ include "helm_lib_csi_image_with_common_fallback" (list . "<raw-container-name>" "<semver>") }} */ -}}
{{- /* returns image name from storage foundation module if enabled, otherwise from common module */ -}}
{{- define "helm_lib_csi_image_with_common_fallback" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $rawContainerName := index . 1 | trimAll "\"" }} {{- /* Container raw name */ -}}
  {{- $kubernetesSemVer := index . 2 }} {{- /* Kubernetes semantic version */ -}}
  {{- $imageDigest := "" }}
  {{- $registryBase := $context.Values.global.modulesImages.registry.base }}
  {{- /* Try to get from storage foundation module if enabled */}}
  {{- if $context.Values.global.enabledModules | has "storage-foundation" }}
    {{- $registryBase = join "/" (list $registryBase "modules" "storage-foundation" ) }}
    {{- $storageFoundationDigests := index $context.Values.global.modulesImages.digests "storageFoundation" | default dict }}
    {{- $currentMinor := int $kubernetesSemVer.Minor }}
    {{- $kubernetesMajor := int $kubernetesSemVer.Major }}
    {{- /* Iterate from currentMinor down to 0: use offset from 0 to currentMinor, then calculate minorVersion = currentMinor - offset */}}
    {{- range $offset := until (int (add $currentMinor 1)) }}
      {{- if not $imageDigest }}
        {{- $minorVersion := int (sub $currentMinor $offset) }}
        {{- $containerName := join "" (list $rawContainerName "ForK8SGE" $kubernetesMajor $minorVersion) }}
        {{- $digest := index $storageFoundationDigests $containerName | default "" }}
        {{- if $digest }}
          {{- $imageDigest = $digest }}
        {{- end }}
      {{- end }}
    {{- end }}
    {{- /* Fallback to base container name if no versioned image found (when minor reached 0) */}}
    {{- if not $imageDigest }}
      {{- $imageDigest = index $storageFoundationDigests $rawContainerName | default "" }}
    {{- end }}
  {{- /* Fallback to common module if storage foundation module is not enabled */}}
  {{- else }}
    {{- $containerName := join "" (list $rawContainerName $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
    {{- $imageDigest = index $context.Values.global.modulesImages.digests "common" $containerName | default "" }}
  {{- end }}
  {{- if $imageDigest }}
    {{- printf "%s@%s" $registryBase $imageDigest }}
  {{- end }}
{{- end }}


{{- define "attacher_resources" }}
cpu: 10m
memory: 25Mi
{{- end }}

{{- define "provisioner_resources" }}
cpu: 10m
memory: 25Mi
{{- end }}

{{- define "resizer_resources" }}
cpu: 10m
memory: 25Mi
{{- end }}

{{- define "syncer_resources" }}
cpu: 10m
memory: 25Mi
{{- end }}

{{- define "snapshotter_resources" }}
cpu: 10m
memory: 25Mi
{{- end }}

{{- define "livenessprobe_resources" }}
cpu: 10m
memory: 25Mi
{{- end }}

{{- define "controller_resources" }}
cpu: 10m
memory: 50Mi
{{- end }}

{{- /* Usage: {{ include "helm_lib_csi_controller_manifests" (list . $config) }} */ -}}
{{- define "helm_lib_csi_controller_manifests" }}
  {{- $context := index . 0 }}

  {{- $config := index . 1 }}
  {{- $fullname := $config.fullname | default "csi-controller" }}
  {{- $snapshotterEnabled := dig "snapshotterEnabled" true $config }}
  {{- $snapshotterSnapshotNamePrefix := dig "snapshotterSnapshotNamePrefix" false $config }}
  {{- $resizerEnabled := dig "resizerEnabled" true $config }}
  {{- $syncerEnabled := dig "syncerEnabled" false $config }}
  {{- $topologyEnabled := dig "topologyEnabled" true $config }}
  {{- $runAsRootUser := dig "runAsRootUser" false $config }}
  {{- $extraCreateMetadataEnabled := dig "extraCreateMetadataEnabled" false $config }}
  {{- $controllerImage := $config.controllerImage | required "$config.controllerImage is required" }}
  {{- $provisionerTimeout := $config.provisionerTimeout | default "600s" }}
  {{- $attacherTimeout := $config.attacherTimeout | default "600s" }}
  {{- $resizerTimeout := $config.resizerTimeout | default "600s" }}
  {{- $snapshotterTimeout := $config.snapshotterTimeout | default "600s" }}
  {{- $provisionerWorkers := $config.provisionerWorkers | default "10" }}
  {{- $volumeNamePrefix := $config.volumeNamePrefix }}
  {{- $volumeNameUUIDLength := $config.volumeNameUUIDLength }}
  {{- $attacherWorkers := $config.attacherWorkers | default "10" }}
  {{- $resizerWorkers := $config.resizerWorkers | default "10" }}
  {{- $snapshotterWorkers := $config.snapshotterWorkers | default "10" }}
  {{- $csiControllerHaMode := $config.csiControllerHaMode | default false }}
  {{- $additionalCsiControllerPodAnnotations := $config.additionalCsiControllerPodAnnotations | default false }}
  {{- $additionalControllerEnvs := $config.additionalControllerEnvs }}
  {{- $additionalSyncerEnvs := $config.additionalSyncerEnvs }}
  {{- $additionalControllerArgs := $config.additionalControllerArgs }}
  {{- $additionalControllerVolumes := $config.additionalControllerVolumes }}
  {{- $additionalControllerVolumeMounts := $config.additionalControllerVolumeMounts }}
  {{- $additionalControllerVPA := $config.additionalControllerVPA }}
  {{- $additionalControllerPorts := $config.additionalControllerPorts }}
  {{- $additionalContainers := $config.additionalContainers }}
  {{- $csiControllerHostNetwork := $config.csiControllerHostNetwork | default "true" }}
  {{- $csiControllerHostPID := $config.csiControllerHostPID | default "false" }}
  {{- $livenessProbePort := $config.livenessProbePort | default 9808 }}
  {{- $initContainers := $config.initContainers }}
  {{- $customNodeSelector := $config.customNodeSelector }}
  {{- $additionalPullSecrets := $config.additionalPullSecrets }}
  {{- $forceCsiControllerPrivilegedContainer := $config.forceCsiControllerPrivilegedContainer | default false }}
  {{- $dnsPolicy := $config.dnsPolicy | default "ClusterFirstWithHostNet" }}

  {{- $kubernetesSemVer := semver $context.Values.global.discovery.kubernetesVersion }}

  {{- $provisionerImage := include "helm_lib_csi_image_with_common_fallback" (list $context "csiExternalProvisioner" $kubernetesSemVer) }}

  {{- $attacherImage := include "helm_lib_csi_image_with_common_fallback" (list $context "csiExternalAttacher" $kubernetesSemVer) }}

  {{- $resizerImage := include "helm_lib_csi_image_with_common_fallback" (list $context "csiExternalResizer" $kubernetesSemVer) }}

  {{- $syncerImageName := join "" (list "csiVsphereSyncer" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $syncerImage := include "helm_lib_module_common_image_no_fail" (list $context $syncerImageName) }}

  {{- $snapshotterImage := include "helm_lib_csi_image_with_common_fallback" (list $context "csiExternalSnapshotter" $kubernetesSemVer) }}

  {{- $livenessprobeImage := include "helm_lib_csi_image_with_common_fallback" (list $context "csiLivenessprobe" $kubernetesSemVer) }}

  {{- if $provisionerImage }}
    {{- if ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" "csi-controller")) | nindent 2 }}
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: Deployment
    name: {{ $fullname }}
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: "provisioner"
      minAllowed:
        {{- include "provisioner_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: 20m
        memory: 50Mi
    - containerName: "attacher"
      minAllowed:
        {{- include "attacher_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: 20m
        memory: 50Mi
    {{- if $resizerEnabled }}
    - containerName: "resizer"
      minAllowed:
        {{- include "resizer_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: 20m
        memory: 50Mi
    {{- end }}
    {{- if $syncerEnabled }}
    - containerName: "syncer"
      minAllowed:
        {{- include "syncer_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: 20m
        memory: 50Mi
    {{- end }}
    {{- if $snapshotterEnabled }}
    - containerName: "snapshotter"
      minAllowed:
        {{- include "snapshotter_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: 20m
        memory: 50Mi
    {{- end }}
    - containerName: "livenessprobe"
      minAllowed:
        {{- include "livenessprobe_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: 20m
        memory: 50Mi
    - containerName: "controller"
      minAllowed:
        {{- include "controller_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: 20m
        memory: 100Mi
    {{- if $additionalControllerVPA }}
    {{- $additionalControllerVPA | toYaml | nindent 4 }}
    {{- end }}
    {{- end }}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" "csi-controller"))  | nindent 2 }}
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: {{ $fullname }}
---
kind: Deployment
apiVersion: apps/v1
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" "csi-controller")) | nindent 2 }}

spec:
  {{- if $csiControllerHaMode }}
  {{- include "helm_lib_deployment_on_master_strategy_and_replicas_for_ha" $context | nindent 2 }}
  {{- else }}
  replicas: 1
  strategy:
    type: Recreate
  {{- end }}
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: {{ $fullname }}
  template:
    metadata:
      labels:
        app: {{ $fullname }}
      {{- if or (hasPrefix "cloud-provider-" $context.Chart.Name) ($additionalCsiControllerPodAnnotations) }}
      annotations:
      {{- if hasPrefix "cloud-provider-" $context.Chart.Name }}
        cloud-config-checksum: {{ include (print $context.Template.BasePath "/cloud-controller-manager/secret.yaml") $context | sha256sum }}
      {{- end }}
      {{- if $additionalCsiControllerPodAnnotations }}
        {{- $additionalCsiControllerPodAnnotations | toYaml | nindent 8 }}
      {{- end }}
      {{- end }}
    spec:
      {{- if $csiControllerHaMode }}
      {{- include "helm_lib_pod_anti_affinity_for_ha" (list $context (dict "app" $fullname)) | nindent 6 }}
      {{- end }}
      hostNetwork: {{ $csiControllerHostNetwork }}
      hostPID: {{ $csiControllerHostPID }}
      {{- if eq $csiControllerHostNetwork "true" }}
      dnsPolicy: {{ $dnsPolicy | quote }}
      {{- end }}
      imagePullSecrets:
      - name: deckhouse-registry
      {{- if $additionalPullSecrets }}
      {{- $additionalPullSecrets | toYaml | nindent 6 }}
      {{- end }}
      {{- include "helm_lib_priority_class" (tuple $context "system-cluster-critical") | nindent 6 }}
      {{- if $customNodeSelector }}
      nodeSelector:
        {{- $customNodeSelector | toYaml | nindent 8 }}
      {{- else }}
      {{- include "helm_lib_node_selector" (tuple $context "master") | nindent 6 }}
      {{- end }}
      {{- include "helm_lib_tolerations" (tuple $context "any-node" "with-uninitialized") | nindent 6 }}
      {{- if $runAsRootUser }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_root" . | nindent 6 }}
      {{- else }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . | nindent 6 }}
      {{- end }}
      serviceAccountName: csi
      automountServiceAccountToken: true
      containers:
      - name: provisioner
        {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true "seccompProfile" true) | nindent 8 }}
        image: {{ $provisionerImage | quote }}
        args:
        - "--timeout={{ $provisionerTimeout }}"
        - "--v=5"
        - "--csi-address=$(ADDRESS)"
  {{- if $volumeNamePrefix }}
        - "--volume-name-prefix={{ $volumeNamePrefix }}"
  {{- end }}
  {{- if $volumeNameUUIDLength }}
        - "--volume-name-uuid-length={{ $volumeNameUUIDLength }}"
  {{- end }}
  {{- if $topologyEnabled }}
        - "--feature-gates=Topology=true"
        - "--strict-topology"
  {{- else }}
        - "--feature-gates=Topology=false"
  {{- end }}
        - "--default-fstype=ext4"
        - "--leader-election=true"
        - "--leader-election-namespace=$(NAMESPACE)"
        - "--leader-election-lease-duration=30s"
        - "--leader-election-renew-deadline=20s"
        - "--leader-election-retry-period=5s"
        - "--enable-capacity"
        - "--capacity-ownerref-level=2"
  {{- if $extraCreateMetadataEnabled }}
        - "--extra-create-metadata=true"
  {{- end }}
        - "--worker-threads={{ $provisionerWorkers }}"
        env:
        - name: ADDRESS
          value: /csi/csi.sock
        - name: POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
  {{- if not ( $context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
            {{- include "provisioner_resources" $context | nindent 12 }}
  {{- end }}
      - name: attacher
        {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true "seccompProfile" true) | nindent 8 }}
        image: {{ $attacherImage | quote }}
        args:
        - "--timeout={{ $attacherTimeout }}"
        - "--v=5"
        - "--csi-address=$(ADDRESS)"
        - "--leader-election=true"
        - "--leader-election-namespace=$(NAMESPACE)"
        - "--leader-election-lease-duration=30s"
        - "--leader-election-renew-deadline=20s"
        - "--leader-election-retry-period=5s"
        - "--worker-threads={{ $attacherWorkers }}"
        env:
        - name: ADDRESS
          value: /csi/csi.sock
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
  {{- if not ( $context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
            {{- include "attacher_resources" $context | nindent 12 }}
  {{- end }}
            {{- if $resizerEnabled }}
      - name: resizer
        {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true "seccompProfile" true) | nindent 8 }}
        image: {{ $resizerImage | quote }}
        args:
        - "--timeout={{ $resizerTimeout }}"
        - "--v=5"
        - "--csi-address=$(ADDRESS)"
        - "--leader-election=true"
        - "--leader-election-namespace=$(NAMESPACE)"
        - "--leader-election-lease-duration=30s"
        - "--leader-election-renew-deadline=20s"
        - "--leader-election-retry-period=5s"
        - "--workers={{ $resizerWorkers }}"
        env:
        - name: ADDRESS
          value: /csi/csi.sock
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
  {{- if not ( $context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
            {{- include "resizer_resources" $context | nindent 12 }}
  {{- end }}
            {{- end }}
            {{- if $syncerEnabled }}
      - name: syncer
        {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true "seccompProfile" true) | nindent 8 }}
        image: {{ $syncerImage | quote }}
        args:
        - "--leader-election"
        - "--leader-election-lease-duration=30s"
        - "--leader-election-renew-deadline=20s"
        - "--leader-election-retry-period=10s"
    {{- if $additionalControllerArgs }}
        {{- $additionalControllerArgs | toYaml | nindent 8 }}
    {{- end }}
    {{- if $additionalSyncerEnvs }}
        env:
        {{- $additionalSyncerEnvs | toYaml | nindent 8 }}
    {{- end }}
    {{- if $additionalControllerVolumeMounts }}
        volumeMounts:
        {{- $additionalControllerVolumeMounts | toYaml | nindent 8 }}
    {{- end }}
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
  {{- if not ( $context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
            {{- include "syncer_resources" $context | nindent 12 }}
  {{- end }}
            {{- end }}
    {{- if $snapshotterEnabled }}
      - name: snapshotter
        {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true "seccompProfile" true) | nindent 8 }}
        image: {{ $snapshotterImage | quote }}
        args:
        - "--timeout={{ $snapshotterTimeout }}"
        - "--v=5"
        - "--csi-address=$(ADDRESS)"
        - "--leader-election=true"
        - "--leader-election-namespace=$(NAMESPACE)"
        - "--leader-election-lease-duration=30s"
        - "--leader-election-renew-deadline=20s"
        - "--leader-election-retry-period=5s"
        - "--worker-threads={{ $snapshotterWorkers }}"
        {{- if $snapshotterSnapshotNamePrefix }}
        - "--snapshot-name-prefix={{ $snapshotterSnapshotNamePrefix }}"
        {{- end }}
        env:
        - name: ADDRESS
          value: /csi/csi.sock
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
        resources:
          requests:
              {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
  {{- if not ( $context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
            {{- include "snapshotter_resources" $context | nindent 12 }}
  {{- end }}
            {{- end }}
      - name: livenessprobe
        {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true "seccompProfile" true) | nindent 8 }}
        image: {{ $livenessprobeImage | quote }}
        args:
        - "--csi-address=$(ADDRESS)"
  {{- if eq $csiControllerHostNetwork "true" }}
        - "--http-endpoint=$(HOST_IP):{{ $livenessProbePort }}"
  {{- else }}
        - "--http-endpoint=$(POD_IP):{{ $livenessProbePort }}"
  {{- end }}
        env:
        - name: ADDRESS
          value: /csi/csi.sock
  {{- if eq $csiControllerHostNetwork "true" }}
        - name: HOST_IP
          valueFrom:
            fieldRef:
              fieldPath: status.hostIP
  {{- else }}
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
  {{- end }}
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
  {{- if not ( $context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
            {{- include "livenessprobe_resources" $context | nindent 12 }}
  {{- end }}
      - name: controller
{{- if $forceCsiControllerPrivilegedContainer }}
        {{- include "helm_lib_module_container_security_context_escalated_sys_admin_privileged" . | nindent 8 }}
{{- else }}
        {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true "seccompProfile" true) | nindent 8 }}
{{- end }}
        image: {{ $controllerImage | quote }}
        args:
    {{- if $additionalControllerArgs }}
        {{- $additionalControllerArgs | toYaml | nindent 8 }}
    {{- end }}
    {{- if $additionalControllerEnvs }}
        env:
        {{- $additionalControllerEnvs | toYaml | nindent 8 }}
    {{- end }}
        livenessProbe:
          httpGet:
            path: /healthz
            port: {{ $livenessProbePort }}
    {{- if $additionalControllerPorts }}
        ports:
        {{- $additionalControllerPorts | toYaml | nindent 8 }}
    {{- end }}
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
        {{- /* For an unknown reason vSphere csi-controller won't start without `/tmp` directory */ -}}
        {{- if eq $context.Chart.Name "cloud-provider-vsphere" }}
        - name: tmp
          mountPath: /tmp
        {{- end }}
    {{- if $additionalControllerVolumeMounts }}
        {{- $additionalControllerVolumeMounts | toYaml | nindent 8 }}
    {{- end }}
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
  {{- if not ( $context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
            {{- include "controller_resources" $context | nindent 12 }}
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

      volumes:
      - name: socket-dir
        emptyDir: {}
      {{- /* For an unknown reason vSphere csi-controller won't start without `/tmp` directory */ -}}
      {{- if eq $context.Chart.Name "cloud-provider-vsphere" }}
      - name: tmp
        emptyDir: {}
      {{- end }}

      {{- if $additionalControllerVolumes }}
        {{- $additionalControllerVolumes | toYaml | nindent 6 }}
      {{- end }}

  {{- end }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_csi_controller_rbac" . }} */ -}}
{{- define "helm_lib_csi_controller_rbac" }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: csi
  namespace: d8-{{ .Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
automountServiceAccountToken: false

# ===========
# provisioner
# ===========
# Source https://github.com/kubernetes-csi/external-provisioner/blob/master/deploy/kubernetes/rbac.yaml
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-provisioner
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
rules:
- apiGroups: [""]
  resources: ["persistentvolumes"]
  verbs: ["get", "list", "watch", "create", "delete"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "watch", "update"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["storage.k8s.io"]
  resources: ["storageclasses"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["list", "watch", "create", "update", "patch"]
- apiGroups: ["snapshot.storage.k8s.io"]
  resources: ["volumesnapshots"]
  verbs: ["get", "list"]
- apiGroups: ["snapshot.storage.k8s.io"]
  resources: ["volumesnapshotcontents"]
  verbs: ["get", "list"]
- apiGroups: ["storage.k8s.io"]
  resources: ["csinodes"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "watch"]
# Access to volumeattachments is only needed when the CSI driver
# has the PUBLISH_UNPUBLISH_VOLUME controller capability.
# In that case, external-provisioner will watch volumeattachments
# to determine when it is safe to delete a volume.
- apiGroups: ["storage.k8s.io"]
  resources: ["volumeattachments"]
  verbs: ["get", "list", "watch"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-provisioner
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
subjects:
- kind: ServiceAccount
  name: csi
  namespace: d8-{{ .Chart.Name }}
roleRef:
  kind: ClusterRole
  name: d8:{{ .Chart.Name }}:csi:controller:external-provisioner
  apiGroup: rbac.authorization.k8s.io
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: csi:controller:external-provisioner
  namespace: d8-{{ .Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
rules:
# Only one of the following rules for endpoints or leases is required based on
# what is set for `--leader-election-type`. Endpoints are deprecated in favor of Leases.
- apiGroups: [""]
  resources: ["endpoints"]
  verbs: ["get", "watch", "list", "delete", "update", "create"]
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "watch", "list", "delete", "update", "create"]
# Permissions for CSIStorageCapacity are only needed enabling the publishing
# of storage capacity information.
- apiGroups: ["storage.k8s.io"]
  resources: ["csistoragecapacities"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# The GET permissions below are needed for walking up the ownership chain
# for CSIStorageCapacity. They are sufficient for deployment via
# StatefulSet (only needs to get Pod) and Deployment (needs to get
# Pod and then ReplicaSet to find the Deployment).
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get"]
- apiGroups: ["apps"]
  resources: ["replicasets"]
  verbs: ["get"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: csi:controller:external-provisioner
  namespace: d8-{{ .Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
subjects:
- kind: ServiceAccount
  name: csi
  namespace: d8-{{ .Chart.Name }}
roleRef:
  kind: Role
  name: csi:controller:external-provisioner
  apiGroup: rbac.authorization.k8s.io

# ========
# attacher
# ========
# Source https://github.com/kubernetes-csi/external-attacher/blob/master/deploy/kubernetes/rbac.yaml
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-attacher
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
rules:
- apiGroups: [""]
  resources: ["persistentvolumes"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["storage.k8s.io"]
  resources: ["csinodes"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["storage.k8s.io"]
  resources: ["volumeattachments"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["storage.k8s.io"]
  resources: ["volumeattachments/status"]
  verbs: ["patch"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-attacher
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
subjects:
- kind: ServiceAccount
  name: csi
  namespace: d8-{{ .Chart.Name }}
roleRef:
  kind: ClusterRole
  name: d8:{{ .Chart.Name }}:csi:controller:external-attacher
  apiGroup: rbac.authorization.k8s.io
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: csi:controller:external-attacher
  namespace: d8-{{ .Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
rules:
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "watch", "list", "delete", "update", "create"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: csi:controller:external-attacher
  namespace: d8-{{ .Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
subjects:
- kind: ServiceAccount
  name: csi
  namespace: d8-{{ .Chart.Name }}
roleRef:
  kind: Role
  name: csi:controller:external-attacher
  apiGroup: rbac.authorization.k8s.io

# =======
# resizer
# =======
# Source https://github.com/kubernetes-csi/external-resizer/blob/master/deploy/kubernetes/rbac.yaml
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-resizer
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
rules:
- apiGroups: [""]
  resources: ["persistentvolumes"]
  verbs: ["get", "list", "watch", "patch"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims/status"]
  verbs: ["patch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["list", "watch", "create", "update", "patch"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-resizer
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
subjects:
- kind: ServiceAccount
  name: csi
  namespace: d8-{{ .Chart.Name }}
roleRef:
  kind: ClusterRole
  name: d8:{{ .Chart.Name }}:csi:controller:external-resizer
  apiGroup: rbac.authorization.k8s.io
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: csi:controller:external-resizer
  namespace: d8-{{ .Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
rules:
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "watch", "list", "delete", "update", "create"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: csi:controller:external-resizer
  namespace: d8-{{ .Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
subjects:
- kind: ServiceAccount
  name: csi
  namespace: d8-{{ .Chart.Name }}
roleRef:
  kind: Role
  name: csi:controller:external-resizer
  apiGroup: rbac.authorization.k8s.io
# ========
# snapshotter
# ========
# Source https://github.com/kubernetes-csi/external-snapshotter/blob/master/deploy/kubernetes/csi-snapshotter/rbac-csi-snapshotter.yaml
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-snapshotter
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
rules:
- apiGroups: [""]
  resources: ["events"]
  verbs: ["list", "watch", "create", "update", "patch"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list"]
- apiGroups: ["snapshot.storage.k8s.io"]
  resources: ["volumesnapshotclasses"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["snapshot.storage.k8s.io"]
  resources: ["volumesnapshotcontents"]
  verbs: ["create", "get", "list", "watch", "update", "delete", "patch"]
- apiGroups: ["snapshot.storage.k8s.io"]
  resources: ["volumesnapshotcontents/status"]
  verbs: ["update", "patch"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-snapshotter
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
subjects:
- kind: ServiceAccount
  name: csi
  namespace: d8-{{ .Chart.Name }}
roleRef:
  kind: ClusterRole
  name: d8:{{ .Chart.Name }}:csi:controller:external-snapshotter
  apiGroup: rbac.authorization.k8s.io
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: csi:controller:external-snapshotter
  namespace: d8-{{ .Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
rules:
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "watch", "list", "delete", "update", "create"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: csi:controller:external-snapshotter
  namespace: d8-{{ .Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | nindent 2 }}
subjects:
- kind: ServiceAccount
  name: csi
  namespace: d8-{{ .Chart.Name }}
roleRef:
  kind: Role
  name: csi:controller:external-snapshotter
  apiGroup: rbac.authorization.k8s.io
{{- end }}
