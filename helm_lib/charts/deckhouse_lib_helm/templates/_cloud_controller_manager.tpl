{{- define "cloud_controller_manager_resources" }}
cpu: 25m
memory: 50Mi
{{- end }}

{{- define "cloud_controller_manager_max_allowed_resources" }}
cpu: 50m
memory: 50Mi
{{- end }}

{{- define "cloud_controller_manager_liveness_probe" -}}
httpGet:
  path: /healthz
  port: 10471
  host: 127.0.0.1
  scheme: HTTPS
{{- end -}}

{{- define "cloud_controller_manager_readiness_probe" -}}
httpGet:
  path: /healthz
  port: 10471
  host: 127.0.0.1
  scheme: HTTPS
{{- end -}}

{{- /* Usage: {{ include "helm_lib_cloud_controller_manager_manifests" (list . $config) }} */ -}}
{{- /* Renders common manifests for provider-specific Cloud Controller Managers. */ -}}
{{- /* Includes Deployment, VerticalPodAutoscaler (optional), PodDisruptionBudget (optional), and SecurityPolicyException (optional). */ -}}
{{- /* Supported configuration parameters: */ -}}
{{- /* + fullname (optional, default: `"cloud-controller-manager"`) — resource base name used for Deployment, PDB, VPA, SecurityPolicyException, and the main container name by default. */ -}}
{{- /* + namespace (optional, default: `d8-{{ $context.Chart.Name }}`) — resource base namespace. */ -}}
{{- /* + image (required) — image for the main container. */ -}}
{{- /* + resources (optional, default: `{cpu: 25m, memory: 50Mi}`) — main container resource requests used when VPA is disabled. */ -}}
{{- /* + priorityClassName (optional, default: `"system-cluster-critical"`) — Pod priority class name. */ -}}
{{- /* + nodeSelectorStrategy (optional, default: `"master"`) — strategy passed to helm_lib_node_selector. */ -}}
{{- /* + tolerationsStrategies (optional, default: ["wildcard"]) — strategies passed to helm_lib_tolerations. */ -}}
{{- /* + hostNetwork (optional, default: `true`) — enables host networking for the Pod and SecurityPolicyException network rule generation. */ -}}
{{- /* + dnsPolicy (optional, default: `"Default"`) — Pod DNS policy. */ -}}
{{- /* + automountServiceAccountToken (optional, default: `true`) — controls whether the service account token is mounted into the Pod. */ -}}
{{- /* + serviceAccountName (optional, default: `$config.fullname`) — ServiceAccount name used by the Pod. */ -}}
{{- /* + revisionHistoryLimit (optional, default: `2`) — number of old ReplicaSets retained by the Deployment. */ -}}
{{- /* + livenessProbe (optional, default: `{httpGet: {path: /healthz, port: 10471, host: 127.0.0.1, scheme: HTTPS}}`) — liveness probe configuration for the main container. */ -}}
{{- /* + readinessProbe (optional, default: `{httpGet: {path: /healthz, port: 10471, host: 127.0.0.1, scheme: HTTPS}}`) — readiness probe configuration for the main container. */ -}}
{{- /* + additionalEnvs (optional, default: `[]`) — extra environment variables for the main container. */ -}}
{{- /* + additionalArgs (optional, default: `nil`) — extra args for the main container. */ -}}
{{- /* + additionalVolumeMounts (optional, default: `[]`) — extra volumeMounts for the main container. */ -}}
{{- /* + additionalVolumes (optional, default: `[]`) — extra Pod volumes; hostPath volumes are also used to build SecurityPolicyException rules when enabled. */ -}}
{{- /* + additionalPodLabels (optional, default: `{}`) — extra labels added to the pod template metadata. */ -}}
{{- /* + additionalPodAnnotations (optional, default: `{}`) — extra annotations added to the pod template metadata. */ -}}
{{- /* + pdbEnabled (optional, default: `true`) — enables PodDisruptionBudget rendering. */ -}}
{{- /* + pdbMaxUnavailable (optional, default: `1`) — maxUnavailable value for PodDisruptionBudget. */ -}}
{{- /* + additionalPDBAnnotations (optional, default: `{}`) — extra annotations added to PodDisruptionBudget metadata. */ -}}
{{- /* + vpaEnabled (optional, default: `true`) — enables VerticalPodAutoscaler rendering. */ -}}
{{- /* + vpaUpdateMode (optional, default: `"InPlaceOrRecreate"`) — VPA update mode. */ -}}
{{- /* + vpaMaxAllowed (optional, default: `{cpu: 50m, memory: 50Mi}`) — maximum resource values used in VPA policy. */ -}}
{{- /* + securityPolicyExceptionEnabled (optional, default: `false`) — enables SecurityPolicyException rendering and adds the related pod label. */ -}}
{{- define "helm_lib_cloud_controller_manager_manifests" }}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc. */ -}}
  {{- $config := index . 1 -}} {{- /* Configuration dict for the Cloud Controller Manager. */ -}}

  {{- $fullname := dig "fullname" "cloud-controller-manager" $config }}
  {{- $namespace := dig "namespace" (printf "d8-%s" $context.Chart.Name) $config -}}
  {{- $image := $config.image | required "image is required" }}
  {{- $resources := dig "resources" (include "cloud_controller_manager_resources" $context | fromYaml) $config }}
  {{- $priorityClassName := dig "priorityClassName" "system-cluster-critical" $config }}
  {{- $nodeSelectorStrategy := dig "nodeSelectorStrategy" "master" $config -}}
  {{- $tolerationsStrategies := dig "tolerationsStrategies" (list "wildcard") $config -}}
  {{- $hostNetwork := dig "hostNetwork" true $config }}
  {{- $dnsPolicy := dig "dnsPolicy" "Default" $config }}
  {{- $automountServiceAccountToken := dig "automountServiceAccountToken" true $config }}
  {{- $serviceAccountName := dig "serviceAccountName" $fullname $config }}
  {{- $revisionHistoryLimit := dig "revisionHistoryLimit" 2 $config }}
  {{- $livenessProbe := dig "livenessProbe" (include "cloud_controller_manager_liveness_probe" $context | fromYaml) $config }}
  {{- $readinessProbe := dig "readinessProbe" (include "cloud_controller_manager_readiness_probe" $context | fromYaml) $config }}
  {{- $additionalEnvs := dig "additionalEnvs" (list) $config }}
  {{- $additionalArgs := dig "additionalArgs" nil $config }}
  {{- $additionalVolumeMounts := dig "additionalVolumeMounts" (list) $config }}
  {{- $additionalVolumes := dig "additionalVolumes" (list) $config }}
  {{- $additionalPodLabels := dig "additionalPodLabels" (dict) $config }}
  {{- $additionalPodAnnotations := dig "additionalPodAnnotations" (dict) $config }}
  {{- $pdbEnabled := dig "pdbEnabled" true $config }}
  {{- $pdbMaxUnavailable := dig "pdbMaxUnavailable" 1 $config }}
  {{- $additionalPDBAnnotations := dig "additionalPDBAnnotations" (dict) $config }}
  {{- $vpaEnabled := dig "vpaEnabled" true $config }}
  {{- $vpaUpdateMode := dig "vpaUpdateMode" "InPlaceOrRecreate" $config }}
  {{- $vpaMaxAllowed := dig "vpaMaxAllowed" (include "cloud_controller_manager_max_allowed_resources" $context | fromYaml) $config }}
  {{- $securityPolicyExceptionEnabled := dig "securityPolicyExceptionEnabled" false $config }}

{{- if and $vpaEnabled ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: {{ $fullname }}
  namespace: {{ $namespace }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $fullname)) | nindent 2 }}
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ $fullname }}
  updatePolicy:
    updateMode: {{ $vpaUpdateMode }}
  resourcePolicy:
    containerPolicies:
      - containerName: {{ $fullname | quote }}
        minAllowed:
          {{- toYaml $resources | nindent 10 }}
        maxAllowed:
          {{- toYaml $vpaMaxAllowed | nindent 10 }}
{{- end }}

{{- if $pdbEnabled }}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ $fullname }}
  namespace: {{ $namespace }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $fullname)) | nindent 2 }}
  {{- with $additionalPDBAnnotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  maxUnavailable: {{ $pdbMaxUnavailable }}
  selector:
    matchLabels:
      app: {{ $fullname }}
{{- end }}

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ $fullname }}
  namespace: {{ $namespace }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $fullname)) | nindent 2 }}
spec:
  {{- include "helm_lib_deployment_on_master_strategy_and_replicas_for_ha" $context | nindent 2 }}
  revisionHistoryLimit: {{ $revisionHistoryLimit }}
  selector:
    matchLabels:
      app: {{ $fullname }}
  template:
    metadata:
      labels:
        app: {{ $fullname }}
      {{- if and $securityPolicyExceptionEnabled ($context.Values.global.enabledModules | has "admission-policy-engine-crd") }}
        security.deckhouse.io/security-policy-exception: {{ $fullname }}
      {{- end }}
      {{- with $additionalPodLabels }}
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with $additionalPodAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
    spec:
      imagePullSecrets:
      - name: deckhouse-registry
      {{- include "helm_lib_pod_anti_affinity_for_ha" (list $context (dict "app" $fullname)) | nindent 6 }}
      {{- include "helm_lib_priority_class" (tuple $context $priorityClassName) | nindent 6 }}
      {{- include "helm_lib_node_selector" (tuple $context $nodeSelectorStrategy) | nindent 6 }}
      {{- include "helm_lib_tolerations" (concat (list $context) $tolerationsStrategies) | nindent 6 }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" $context | nindent 6 }}
      automountServiceAccountToken: {{ $automountServiceAccountToken }}
      hostNetwork: {{ $hostNetwork }}
      dnsPolicy: {{ $dnsPolicy }}
      serviceAccountName: {{ $serviceAccountName }}
      containers:
        - name: {{ $fullname }}
          {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" dict | nindent 10 }}
          image: {{ $image }}
          args:
            - --leader-elect=true
            - --bind-address=127.0.0.1
            - --secure-port=10471
          {{- with $additionalArgs }}
            {{- toYaml . | nindent 12 }}
          {{- end }}
          env:
            {{- if not $context.Values.global.clusterIsBootstrapped }}
            - name: KUBERNETES_SERVICE_HOST
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: status.hostIP
            - name: KUBERNETES_SERVICE_PORT
              value: "6443"
            {{- end }}
            - name: HOST_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.hostIP
          {{- with $additionalEnvs }}
            {{- toYaml . | nindent 12 }}
          {{- end }}
            {{- include "helm_lib_envs_for_proxy" $context | nindent 12 }}
          livenessProbe:
          {{- with $livenessProbe }}
            {{- toYaml . | nindent 12 }}
          {{- end }}
          readinessProbe:
          {{- with $readinessProbe }}
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with $additionalVolumeMounts }}
          volumeMounts:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          resources:
            requests:
              {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 14 }}
            {{- if not (and $vpaEnabled ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd")) }}
              {{- toYaml $resources | nindent 14 }}
            {{- end }}
      {{- with $additionalVolumes }}
      volumes:
        {{- toYaml . | nindent 8 }}
      {{- end }}

{{- if and $securityPolicyExceptionEnabled ($context.Values.global.enabledModules | has "admission-policy-engine-crd") }}
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicyException
metadata:
  name: {{ $fullname }}
  namespace: {{ $namespace }}
spec:
  {{- if $hostNetwork }}
  network:
    hostNetwork:
      allowedValue: true
      metadata:
        description: |
          Allow host network access for Cloud Controller Manager.
          The Cloud Controller Manager requires host network access to communicate with the API for managing infrastructure resources, including load balancer configuration, node lifecycle management, and routing operations.
  {{- end }}

  {{- $hasHostPathVolumes := false }}
  {{- if $additionalVolumes }}
    {{- range $volume := $additionalVolumes }}
      {{- if $volume.hostPath }}
        {{- $hasHostPathVolumes = true }}
      {{- end }}
    {{- end }}
  {{- end }}
  {{- if $hasHostPathVolumes }}
  volumes:
    types:
      allowedValues:
        - hostPath
      metadata:
        description: |
          Allow hostPath volume type for Cloud Controller Manager.
          The Cloud Controller Manager requires hostPath volumes for accessing host-level resources needed for cloud provider integration and infrastructure management operations.
    hostPath:
      allowedValues:
      {{- range $volume := $additionalVolumes }}
        {{- if $volume.hostPath }}
          {{- $readOnly := false }}
          {{- range $volumeMount := $additionalVolumeMounts }}
            {{- if eq $volumeMount.name $volume.name }}
              {{- $readOnly = (default false $volumeMount.readOnly) }}
            {{- end }}
          {{- end }}
        - path: {{ $volume.hostPath.path }}
          readOnly: {{ $readOnly }}
          metadata:
            description: |
              Allow access to additional hostPath volume at {{ $volume.hostPath.path }}.
              This additional hostPath volume is required by the Cloud Controller Manager for provider-specific infrastructure management operations.
        {{- end }}
      {{- end }}
  {{- end }}
{{- end }}

{{- end }}
