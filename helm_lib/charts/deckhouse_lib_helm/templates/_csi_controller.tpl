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
  {{- $resizerEnabled := dig "resizerEnabled" true $config }}
  {{- $syncerEnabled := dig "syncerEnabled" false $config }}
  {{- $topologyEnabled := dig "topologyEnabled" true $config }}
  {{- $extraCreateMetadataEnabled := dig "extraCreateMetadataEnabled" false $config }}
  {{- $controllerImage := $config.controllerImage | required "$config.controllerImage is required" }}
  {{- $provisionerTimeout := $config.provisionerTimeout | default "600s" }}
  {{- $attacherTimeout := $config.attacherTimeout | default "600s" }}
  {{- $resizerTimeout := $config.resizerTimeout | default "600s" }}
  {{- $snapshotterTimeout := $config.snapshotterTimeout | default "600s" }}
  {{- $provisionerWorkers := $config.provisionerWorkers | default "10" }}
  {{- $attacherWorkers := $config.attacherWorkers | default "10" }}
  {{- $resizerWorkers := $config.resizerWorkers | default "10" }}
  {{- $snapshotterWorkers := $config.snapshotterWorkers | default "10" }}
  {{- $additionalControllerEnvs := $config.additionalControllerEnvs }}
  {{- $additionalSyncerEnvs := $config.additionalSyncerEnvs }}
  {{- $additionalControllerArgs := $config.additionalControllerArgs }}
  {{- $additionalControllerVolumes := $config.additionalControllerVolumes }}
  {{- $additionalControllerVolumeMounts := $config.additionalControllerVolumeMounts }}
  {{- $additionalContainers := $config.additionalContainers }}
  {{- $livenessProbePort := $config.livenessProbePort | default 9808 }}

  {{- $kubernetesSemVer := semver $context.Values.global.discovery.kubernetesVersion }}

  {{- $provisionerImageName := join "" (list "csiExternalProvisioner" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $provisionerImage := include "helm_lib_module_common_image_no_fail" (list $context $provisionerImageName) }}

  {{- $attacherImageName := join "" (list "csiExternalAttacher" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $attacherImage := include "helm_lib_module_common_image_no_fail" (list $context $attacherImageName) }}

  {{- $resizerImageName := join "" (list "csiExternalResizer" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $resizerImage := include "helm_lib_module_common_image_no_fail" (list $context $resizerImageName) }}

  {{- $syncerImageName := join "" (list "csiVsphereSyncer" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $syncerImage := include "helm_lib_module_common_image_no_fail" (list $context $syncerImageName) }}

  {{- $snapshotterImageName := join "" (list "csiExternalSnapshotter" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $snapshotterImage := include "helm_lib_module_common_image_no_fail" (list $context $snapshotterImageName) }}

  {{- $livenessprobeImageName := join "" (list "csiLivenessprobe" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $livenessprobeImage := include "helm_lib_module_common_image_no_fail" (list $context $livenessprobeImageName) }}

  {{- if $provisionerImage }}
    {{- if ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" "csi-controller" "workload-resource-policy.deckhouse.io" "master")) | nindent 2 }}
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
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: {{ $fullname }}
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: {{ $fullname }}
    {{- if hasPrefix "cloud-provider-" $context.Chart.Name }}
      annotations:
        cloud-config-checksum: {{ include (print $context.Template.BasePath "/cloud-controller-manager/secret.yaml") $context | sha256sum }}
    {{- end }}
    spec:
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      imagePullSecrets:
      - name: deckhouse-registry
      {{- include "helm_lib_priority_class" (tuple $context "system-cluster-critical") | nindent 6 }}
      {{- include "helm_lib_node_selector" (tuple $context "master") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple $context "any-node" "with-uninitialized") | nindent 6 }}
{{- if $context.Values.global.enabledModules | has "csi-nfs" }}
      {{- include "helm_lib_module_pod_security_context_runtime_default" . | nindent 6 }}
{{- else }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . | nindent 6 }}
{{- end }}
      serviceAccountName: csi
      containers:
      - name: provisioner
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem" . | nindent 8 }}
        image: {{ $provisionerImage | quote }}
        args:
        - "--timeout={{ $provisionerTimeout }}"
        - "--v=5"
        - "--csi-address=$(ADDRESS)"
  {{- if $topologyEnabled }}
        - "--feature-gates=Topology=true"
        - "--strict-topology"
  {{- else }}
        - "--feature-gates=Topology=false"
  {{- end }}
        - "--default-fstype=ext4"
        - "--leader-election=true"
        - "--leader-election-namespace=$(NAMESPACE)"
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
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem" . | nindent 8 }}
        image: {{ $attacherImage | quote }}
        args:
        - "--timeout={{ $attacherTimeout }}"
        - "--v=5"
        - "--csi-address=$(ADDRESS)"
        - "--leader-election=true"
        - "--leader-election-namespace=$(NAMESPACE)"
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
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem" . | nindent 8 }}
        image: {{ $resizerImage | quote }}
        args:
        - "--timeout={{ $resizerTimeout }}"
        - "--v=5"
        - "--csi-address=$(ADDRESS)"
        - "--leader-election=true"
        - "--leader-election-namespace=$(NAMESPACE)"
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
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem" . | nindent 8 }}
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
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem" . | nindent 8 }}
        image: {{ $snapshotterImage | quote }}
        args:
        - "--timeout={{ $snapshotterTimeout }}"
        - "--v=5"
        - "--csi-address=$(ADDRESS)"
        - "--leader-election=true"
        - "--leader-election-namespace=$(NAMESPACE)"
        - "--worker-threads={{ $snapshotterWorkers }}"
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
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem" . | nindent 8 }}
        image: {{ $livenessprobeImage | quote }}
        args:
        - "--csi-address=$(ADDRESS)"
        - "--http-endpoint=$(HOST_IP):{{ $livenessProbePort }}"
        env:
        - name: ADDRESS
          value: /csi/csi.sock
        - name: HOST_IP
          valueFrom:
            fieldRef:
              fieldPath: status.hostIP
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
{{- if $context.Values.global.enabledModules | has "csi-nfs" }}
        {{- include "helm_lib_module_container_security_context_escalated_sys_admin_privileged" . | nindent 8 }}
{{- else }}
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem" . | nindent 8 }}
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
