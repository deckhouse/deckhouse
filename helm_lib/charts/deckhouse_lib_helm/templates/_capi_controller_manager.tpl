{{- define "capi_controller_manager_resources" -}}
cpu: 25m
memory: 50Mi
{{- end -}}

{{- define "capi_controller_manager_max_allowed_resources" -}}
cpu: 50m
memory: 50Mi
{{- end -}}

{{- define "capi_controller_manager_liveness_probe" -}}
httpGet:
  path: /healthz
  port: 8081
initialDelaySeconds: 15
periodSeconds: 20
{{- end -}}

{{- define "capi_controller_manager_readiness_probe" -}}
httpGet:
  path: /readyz
  port: 8081
initialDelaySeconds: 5
periodSeconds: 10
{{- end -}}

{{- /* Usage: {{ include "helm_lib_capi_controller_manager_manifests" (list . $config) }} */ -}}
{{- /* Renders common manifests for provider-specific CAPI Controller Managers. */ -}}
{{- /* Includes Deployment, VerticalPodAutoscaler (optional) and PodDisruptionBudget (optional). */ -}}
{{- /* Supported configuration parameters: */ -}}
{{- /* + fullname (required) — resource base name used for Deployment, PDB, VPA, and by default for the main container name. */ -}}
{{- /* + namespace (optional, default: `d8-{{ $context.Chart.Name }}`) — resource base namespace. */ -}}
{{- /* + image (required) — image for the main container. */ -}}
{{- /* + capiProviderName (required) — value for the cluster.x-k8s.io/provider label in selectors and pod labels. */ -}}
{{- /* + resources (optional, default: `{cpu: 25m, memory: 50Mi}`) — main container resource requests used when VPA is disabled. */ -}}
{{- /* + priorityClassName (optional, default: `"system-cluster-critical"`) — Pod priority class name. */ -}}
{{- /* + serviceAccountName (optional, default: `$config.fullname`) — ServiceAccount name used by the Pod. */ -}}
{{- /* + automountServiceAccountToken (optional, default: `true`) — controls whether the service account token is mounted into the Pod. */ -}}
{{- /* + revisionHistoryLimit (optional, default: `2`) — number of old ReplicaSets retained by the Deployment. */ -}}
{{- /* + terminationGracePeriodSeconds (optional, default: `10`) — Pod termination grace period. */ -}}
{{- /* + hostNetwork (optional, default: `false`) — enables host networking for the Pod. */ -}}
{{- /* + dnsPolicy (optional, default: `nil`) — Pod DNS policy; if not set, the field is omitted. */ -}}
{{- /* + nodeSelectorStrategy (optional, default: `"master"`) — strategy passed to helm_lib_node_selector. */ -}}
{{- /* + tolerationsStrategies (optional, default: `["any-node", "uninitialized"]`) — arguments passed to helm_lib_tolerations. */ -}}
{{- /* + livenessProbe (optional, default: `{httpGet: {path: /healthz, port: 8081}, initialDelaySeconds: 15, periodSeconds: 20}`) — liveness probe configuration for the main container. */ -}}
{{- /* + readinessProbe (optional, default: `{httpGet: {path: /readyz, port: 8081}, initialDelaySeconds: 5, periodSeconds: 10}`) — readiness probe configuration for the main container. */ -}}
{{- /* + additionalArgs (optional, default: `[]`) — extra args for the main container. */ -}}
{{- /* + additionalEnv (optional, default: `[]`) — extra environment variables for the main container. */ -}}
{{- /* + additionalPorts (optional, default: `[]`) — extra container ports for the main container. */ -}}
{{- /* + additionalInitContainers (optional, default: `[]`) — extra initContainers for the Pod. */ -}}
{{- /* + additionalVolumeMounts (optional, default: `[]`) — extra volumeMounts for the main container. */ -}}
{{- /* + additionalVolumes (optional, default: `[]`) — extra Pod volumes. */ -}}
{{- /* + additionalPodLabels (optional, default: `{}`) — extra labels added to the pod template metadata. */ -}}
{{- /* + additionalPodAnnotations (optional, default: `{}`) — extra annotations added to the pod template metadata. */ -}}
{{- /* + pdbEnabled (optional, default: `true`) — enables PodDisruptionBudget rendering. */ -}}
{{- /* + pdbMaxUnavailable (optional, default: `1`) — maxUnavailable value for PodDisruptionBudget. */ -}}
{{- /* + vpaEnabled (optional, default: `false`) — enables VerticalPodAutoscaler rendering. */ -}}
{{- /* + vpaUpdateMode (optional, default: `"InPlaceOrRecreate"`) — VPA update mode. */ -}}
{{- /* + vpaMaxAllowed (optional, default: `{cpu: 50m, memory: 50Mi}`) — maximum resource values used in VPA policy. */ -}}
{{- /* + securityPolicyExceptionEnabled (optional, default: `false`) — enables SecurityPolicyException rendering and adds the related pod label. */ -}}
{{- define "helm_lib_capi_controller_manager_manifests" -}}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc. */ -}}
  {{- $config := index . 1 -}} {{- /* Configuration dict for the CAPI Controller Manager. */ -}}

  {{- $fullname := required "helm_lib_capi_controller_manager_manifests: fullname is required" $config.fullname -}}
  {{- $namespace := dig "namespace" (printf "d8-%s" $context.Chart.Name) $config -}}
  {{- $image := required "helm_lib_capi_controller_manager_manifests: image is required" $config.image -}}
  {{- $capiProviderName := required "helm_lib_capi_controller_manager_manifests: $capiProviderName is required" $config.capiProviderName -}}
  {{- $resources := dig "resources" (include "capi_controller_manager_resources" $context | fromYaml) $config -}}
  {{- $priorityClassName := dig "priorityClassName" "system-cluster-critical" $config -}}
  {{- $serviceAccountName := dig "serviceAccountName" $fullname $config -}}
  {{- $automountServiceAccountToken := dig "automountServiceAccountToken" true $config -}}
  {{- $revisionHistoryLimit := dig "revisionHistoryLimit" 2 $config -}}
  {{- $terminationGracePeriodSeconds := dig "terminationGracePeriodSeconds" 10 $config -}}
  {{- $hostNetwork := dig "hostNetwork" false $config -}}
  {{- $dnsPolicy := dig "dnsPolicy" nil $config -}}
  {{- $nodeSelectorStrategy := dig "nodeSelectorStrategy" "master" $config -}}
  {{- $tolerationsStrategies := dig "tolerationsStrategies" (list "any-node" "uninitialized") $config -}}
  {{- $livenessProbe := dig "livenessProbe" (include "capi_controller_manager_liveness_probe" $context | fromYaml) $config }}
  {{- $readinessProbe := dig "readinessProbe" (include "capi_controller_manager_readiness_probe" $context | fromYaml) $config }}
  {{- $additionalArgs := dig "additionalArgs" (list) $config -}}
  {{- $additionalEnv := dig "additionalEnv" (list) $config -}}
  {{- $additionalPorts := dig "additionalPorts" (list) $config -}}
  {{- $additionalInitContainers := dig "additionalInitContainers" (list) $config -}}
  {{- $additionalVolumeMounts := dig "additionalVolumeMounts" (list) $config -}}
  {{- $additionalVolumes := dig "additionalVolumes" (list) $config -}}
  {{- $additionalPodLabels := dig "additionalPodLabels" (dict) $config -}}
  {{- $additionalPodAnnotations := dig "additionalPodAnnotations" (dict) $config -}}
  {{- $pdbEnabled := dig "pdbEnabled" true $config -}}
  {{- $pdbMaxUnavailable := dig "pdbMaxUnavailable" 1 $config -}}
  {{- $vpaEnabled := dig "vpaEnabled" false $config -}}
  {{- $vpaUpdateMode := dig "vpaUpdateMode" "InPlaceOrRecreate" $config -}}
  {{- $vpaMaxAllowed := dig "vpaMaxAllowed" (include "capi_controller_manager_max_allowed_resources" $context | fromYaml) $config -}}
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
    updateMode: {{ $vpaUpdateMode | quote }}
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
      cluster.x-k8s.io/provider: {{ $capiProviderName }}
      control-plane: controller-manager
  template:
    metadata:
      labels:
        app: {{ $fullname }}
        cluster.x-k8s.io/provider: {{ $capiProviderName }}
        control-plane: controller-manager
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
      serviceAccountName: {{ $serviceAccountName }}
      terminationGracePeriodSeconds: {{ $terminationGracePeriodSeconds }}
      hostNetwork: {{ $hostNetwork }}
      {{- with $dnsPolicy }}
      dnsPolicy: {{ . }}
      {{- end }}
      {{- with $additionalInitContainers }}
      initContainers:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
      - name: {{ $fullname }}
        {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" dict | nindent 8 }}
        image: {{ $image }}
        imagePullPolicy: IfNotPresent
        args:
          - --leader-elect
        {{- with $additionalArgs }}
          {{- toYaml . | nindent 10 }}
        {{- end }}
        {{- with $additionalEnv }}
        env:
          {{- toYaml . | nindent 10 }}
        {{- end }}
        {{- with $additionalPorts }}
        ports:
          {{- toYaml . | nindent 10 }}
        {{- end }}
        livenessProbe:
        {{- with $livenessProbe }}
          {{- toYaml . | nindent 10 }}
        {{- end }}
        readinessProbe:
        {{- with $readinessProbe }}
          {{- toYaml . | nindent 10 }}
        {{- end }}
        {{- with $additionalVolumeMounts }}
        volumeMounts:
          {{- toYaml . | nindent 10 }}
        {{- end }}
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 12 }}
          {{- if not (and $vpaEnabled ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd")) }}
            {{- toYaml $resources | nindent 12 }}
          {{- end }}
      {{- with $additionalVolumes }}
      volumes:
        {{- toYaml . | nindent 8 }}
      {{- end }}

{{- if and $securityPolicyExceptionEnabled ($context.Values.global.enabledModules | has "admission-policy-engine-crd") }}
{{- if $hostNetwork }}
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
          Allow host network access for CAPI infrastructure controller manager.
          The CAPI infrastructure controller manager requires host network access to continue infrastructure reconciliation even if the CNI or pod network is unavailable.
  {{- end }}
{{- end }}
{{- end }}

{{- end -}}