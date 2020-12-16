{{- /* Usage: {{ include "helm_lib_csi_controller_manifests" (list . $config) }} */ -}}
{{- define "helm_lib_csi_controller_manifests" }}
  {{- $context := index . 0 }}

  {{- $config := index . 1 }}
  {{- $controllerImage := $config.controllerImage | required "$config.controllerImage is required" }}
  {{- $provisionerTimeout := $config.provisionerTimeout | default "600s" }}
  {{- $attacherTimeout := $config.attacherTimeout | default "600s" }}
  {{- $resizerTimeout := $config.resizerTimeout | default "600s" }}
  {{- $additionalControllerEnvs := $config.additionalControllerEnvs }}
  {{- $additionalControllerArgs := $config.additionalControllerArgs }}
  {{- $additionalControllerVolumes := $config.additionalControllerVolumes }}
  {{- $additionalControllerVolumeMounts := $config.additionalControllerVolumeMounts }}

  {{- $kubernetesSemVer := semver $context.Values.global.discovery.kubernetesVersion }}

  {{- $provisionerImageName := join "" (list "csiExternalProvisioner" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $provisionerImageTag := index $context.Values.global.modulesImages.tags.common $provisionerImageName }}
  {{- $provisionerImage := printf "%s/common/csi-external-provisioner-%v-%v:%s" $context.Values.global.modulesImages.registry $kubernetesSemVer.Major $kubernetesSemVer.Minor $provisionerImageTag }}

  {{- $attacherImageName := join "" (list "csiExternalAttacher" $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
  {{- $attacherImageTag := index $context.Values.global.modulesImages.tags.common $attacherImageName }}
  {{- $attacherImage := printf "%s/common/csi-external-attacher-%v-%v:%s" $context.Values.global.modulesImages.registry $kubernetesSemVer.Major $kubernetesSemVer.Minor $attacherImageTag }}

  {{- $resizerImageTag := index $context.Values.global.modulesImages.tags.common "csiExternalResizer" }}
  {{- $resizerImage := printf "%s/common/csi-external-resizer:%s" $context.Values.global.modulesImages.registry $resizerImageTag }}

  {{- if $provisionerImageTag }}
    {{- if ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
---
apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  name: csi-controller
  namespace: d8-{{ $context.Chart.Name }}
{{ include "helm_lib_module_labels" (list $context (dict "app" "csi-controller" "workload-resource-policy.deckhouse.io" "master")) | indent 2 }}
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: StatefulSet
    name: csi-controller
  updatePolicy:
    updateMode: "Auto"
    {{- end }}
---
kind: StatefulSet
apiVersion: apps/v1
metadata:
  name: csi-controller
  namespace: d8-{{ $context.Chart.Name }}
{{ include "helm_lib_module_labels" (list $context (dict "app" "csi-controller")) | indent 2 }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: csi-controller
  serviceName: ""
  template:
    metadata:
      labels:
        app: csi-controller
    spec:
      imagePullSecrets:
      - name: deckhouse-registry
{{ include "helm_lib_priority_class" (tuple $context "cluster-critical") | indent 6 }}
{{ include "helm_lib_node_selector" (tuple $context "master") | indent 6 }}
{{ include "helm_lib_tolerations" (tuple $context "master") | indent 6 }}
      serviceAccountName: csi
      containers:
      - name: provisioner
        image: {{ $provisionerImage | quote }}
        args:
        - "--timeout={{ $provisionerTimeout }}"
        - "--v=5"
        - "--csi-address=/csi/csi.sock"
        - "--feature-gates=Topology=true"
        - "--strict-topology"
  {{- if semverCompare ">= 1.19" $context.Values.global.discovery.kubernetesVersion }}
        - "--default-fstype=ext4"
  {{- end }}
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
      - name: attacher
        image: {{ $attacherImage | quote }}
        args:
        - "--timeout={{ $attacherTimeout }}"
        - "--v=5"
        - "--csi-address=/csi/csi.sock"
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
      - name: resizer
        image: {{ $resizerImage | quote }}
        args:
        - "--timeout={{ $resizerTimeout }}"
        - "--v=5"
        - "--csi-address=/csi/csi.sock"
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
      - name: controller
        image: {{ $controllerImage | quote }}
        args:
    {{- if $additionalControllerArgs }}
{{ $additionalControllerArgs | toYaml | indent 8 }}
    {{- end }}
    {{- if $additionalControllerEnvs }}
        env:
{{ $additionalControllerEnvs | toYaml | indent 8 }}
    {{- end }}
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
    {{- if $additionalControllerVolumeMounts }}
{{ $additionalControllerVolumeMounts | toYaml | indent 8 }}
    {{- end }}
      volumes:
      - name: socket-dir
        emptyDir: {}
    {{- if $additionalControllerVolumes }}
{{ $additionalControllerVolumes | toYaml | indent 6 }}
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
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}

# ===========
# provisioner
# ===========
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-provisioner
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
rules:
- apiGroups: [""]
  resources: ["persistentvolumes"]
  verbs: ["get", "list", "watch", "create", "delete"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "watch", "update"]
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
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
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
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
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
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
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
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-attacher
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
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
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
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
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
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
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
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
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ .Chart.Name }}:csi:controller:external-resizer
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
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
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
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
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
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
{{ include "helm_lib_module_labels" (list . (dict "app" "csi-controller")) | indent 2 }}
subjects:
- kind: ServiceAccount
  name: csi
  namespace: d8-{{ .Chart.Name }}
roleRef:
  kind: Role
  name: csi:controller:external-resizer
  apiGroup: rbac.authorization.k8s.io
{{- end }}
