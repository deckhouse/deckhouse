{{- /* Usage: {{ include "helm_lib_module_controller_resources" . }} */ -}}
{{- /* Returns default controller resources */ -}}
{{- define "helm_lib_module_controller_resources" }}
cpu: 10m
memory: 25Mi
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_webhooks_resources" . }} */ -}}
{{- /* Returns default webhooks resources */ -}}
{{- define "helm_lib_module_webhooks_resources" }}
cpu: 10m
memory: 50Mi
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_controller_manifests" (list . $config) }} */ -}}
{{- /*
  Generates controller Deployment, VPA and PDB manifests.

  $config parameters:
  - fullname: name for the deployment (default: "controller")
  - valuesKey: key to access module values (e.g., "csiHpe", "csiS3")
  - webhookEnabled: whether to include webhook container (default: false)
  - webhookCertPath: path to webhook cert in values (e.g., "internal.customWebhookCert")
  - onMasterNode: use master node selector and tolerations (default: true)
  - priorityClass: priority class name (default: "system-cluster-critical")
  - podSecurityContext: "deckhouse" or "nobody" (default: "nobody")
  - additionalLabels: additional labels for VPA
  - controllerMaxCpu: max CPU for controller VPA (default: "200m")
  - controllerMaxMemory: max memory for controller VPA (default: "100Mi")
  - webhooksMaxCpu: max CPU for webhooks VPA (default: "20m")
  - webhooksMaxMemory: max memory for webhooks VPA (default: "100Mi")
  - additionalContainers: additional containers to add to the pod
  - additionalVolumes: additional volumes to add to the pod
  - additionalControllerVolumeMounts: additional volume mounts for controller
  - additionalControllerEnvs: additional environment variables for controller
  - controllerImage: custom controller image (default: uses helm_lib_module_image)
  - controllerImageName: image name for helm_lib_module_image (default: "controller")
  - webhooksImageName: image name for webhooks (default: "webhooks")
  - webhooksPort: port for webhooks container (default: 8443)
  - webhooksCertMountPath: mount path for webhook certs (default: "/etc/webhook/certs")
  - webhooksCommand: command for webhooks container
  - controllerPort: port for controller probes (default: 8081)
  - controllerMetricsPort: port for controller metrics (optional, no port exposed if not set)
*/ -}}
{{- define "helm_lib_module_controller_manifests" }}
  {{- $context := index . 0 }}
  {{- $config := index . 1 }}
  
  {{- $fullname := $config.fullname | default "controller" }}
  {{- $valuesKey := $config.valuesKey | required "$config.valuesKey is required" }}
  {{- $webhookEnabled := dig "webhookEnabled" false $config }}
  {{- $webhookCertPath := $config.webhookCertPath | default "internal.customWebhookCert" }}
  {{- $onMasterNode := dig "onMasterNode" true $config }}
  {{- $priorityClass := $config.priorityClass | default "system-cluster-critical" }}
  {{- $podSecurityContext := $config.podSecurityContext | default "nobody" }}
  {{- $additionalLabels := $config.additionalLabels | default dict }}
  {{- $controllerMaxCpu := $config.controllerMaxCpu | default "200m" }}
  {{- $controllerMaxMemory := $config.controllerMaxMemory | default "100Mi" }}
  {{- $webhooksMaxCpu := $config.webhooksMaxCpu | default "20m" }}
  {{- $webhooksMaxMemory := $config.webhooksMaxMemory | default "100Mi" }}
  {{- $additionalContainers := $config.additionalContainers }}
  {{- $additionalVolumes := $config.additionalVolumes }}
  {{- $additionalControllerVolumeMounts := $config.additionalControllerVolumeMounts }}
  {{- $additionalControllerEnvs := $config.additionalControllerEnvs }}
  {{- $controllerImage := $config.controllerImage }}
  {{- $controllerImageName := $config.controllerImageName | default "controller" }}
  {{- $webhooksImageName := $config.webhooksImageName | default "webhooks" }}
  {{- $webhooksPort := $config.webhooksPort | default 8443 }}
  {{- $webhooksCertMountPath := $config.webhooksCertMountPath | default "/etc/webhook/certs" }}
  {{- $webhooksCommand := $config.webhooksCommand }}
  {{- $controllerPort := $config.controllerPort | default 8081 }}
  {{- $controllerMetricsPort := $config.controllerMetricsPort }}

  {{- /* Get module values */ -}}
  {{- $moduleValues := index $context.Values $valuesKey }}

  {{- /* Build VPA labels */ -}}
  {{- $vpaLabels := dict "app" $fullname }}
  {{- if $onMasterNode }}
    {{- $vpaLabels = merge $vpaLabels (dict "workload-resource-policy.deckhouse.io" "master") }}
  {{- end }}
  {{- $vpaLabels = merge $vpaLabels $additionalLabels }}

  {{- if ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context $vpaLabels) | nindent 2 }}
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: Deployment
    name: {{ $fullname }}
  updatePolicy:
    updateMode: "Initial"
  resourcePolicy:
    containerPolicies:
    - containerName: "controller"
      minAllowed:
        {{- include "helm_lib_module_controller_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: {{ $controllerMaxCpu }}
        memory: {{ $controllerMaxMemory }}
    {{- if $webhookEnabled }}
    - containerName: "webhooks"
      minAllowed:
        {{- include "helm_lib_module_webhooks_resources" $context | nindent 8 }}
      maxAllowed:
        cpu: {{ $webhooksMaxCpu }}
        memory: {{ $webhooksMaxMemory }}
    {{- end }}
  {{- end }}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $fullname)) | nindent 2 }}
spec:
  minAvailable: {{ include "helm_lib_is_ha_to_value" (list $context 1 0) }}
  selector:
    matchLabels:
      app: {{ $fullname }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $fullname)) | nindent 2 }}
spec:
  revisionHistoryLimit: 2
  {{- if $onMasterNode }}
  {{- include "helm_lib_deployment_on_master_strategy_and_replicas_for_ha" $context | nindent 2 }}
  {{- else }}
  {{- include "helm_lib_deployment_strategy_and_replicas_for_ha" $context | nindent 2 }}
  {{- end }}
  selector:
    matchLabels:
      app: {{ $fullname }}
  template:
    metadata:
      {{- if $webhookEnabled }}
      {{- $certPath := printf "%s.ca" $webhookCertPath }}
      {{- $certValue := $moduleValues }}
      {{- range $part := (split "." $certPath) }}
        {{- $certValue = index $certValue $part }}
      {{- end }}
      annotations:
        checksum/ca: {{ $certValue | sha256sum | quote }}
      {{- end }}
      labels:
        app: {{ $fullname }}
    spec:
      {{- include "helm_lib_priority_class" (tuple $context $priorityClass) | nindent 6 }}
      {{- if $onMasterNode }}
      {{- include "helm_lib_tolerations" (tuple $context "any-node" "with-uninitialized" "with-cloud-provider-uninitialized") | nindent 6 }}
      {{- include "helm_lib_node_selector" (tuple $context "master") | nindent 6 }}
      {{- else }}
      {{- include "helm_lib_node_selector" (tuple $context "system") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple $context "system") | nindent 6 }}
      {{- end }}
      {{- include "helm_lib_pod_anti_affinity_for_ha" (list $context (dict "app" $fullname)) | nindent 6 }}
      {{- if eq $podSecurityContext "deckhouse" }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" $context | nindent 6 }}
      {{- else }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_nobody" $context | nindent 6 }}
      {{- end }}
      imagePullSecrets:
        - name: {{ $context.Chart.Name }}-module-registry
      serviceAccountName: {{ $fullname }}
      containers:
        - name: controller
          {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true "seccompProfile" true) | nindent 10 }}
          {{- if $controllerImage }}
          image: {{ $controllerImage | quote }}
          {{- else }}
          image: {{ include "helm_lib_module_image" (list $context $controllerImageName) }}
          {{- end }}
          imagePullPolicy: IfNotPresent
          readinessProbe:
            httpGet:
              path: /readyz
              port: {{ $controllerPort }}
              scheme: HTTP
            initialDelaySeconds: 5
            failureThreshold: 2
            periodSeconds: 1
          livenessProbe:
            httpGet:
              path: /healthz
              port: {{ $controllerPort }}
              scheme: HTTP
            periodSeconds: 1
            failureThreshold: 3
          {{- if $controllerMetricsPort }}
          ports:
            - name: metrics
              containerPort: {{ $controllerMetricsPort }}
              protocol: TCP
          {{- end }}
          resources:
            requests:
              {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 14 }}
{{- if not ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
              {{- include "helm_lib_module_controller_resources" $context | nindent 14 }}
{{- end }}
          env:
            - name: LOG_LEVEL
              value: {{ include "helm_lib_module_controller_log_level" (list $context $valuesKey) | quote }}
            - name: CONTROLLER_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            {{- if $additionalControllerEnvs }}
            {{- $additionalControllerEnvs | toYaml | nindent 12 }}
            {{- end }}
          {{- if $additionalControllerVolumeMounts }}
          volumeMounts:
            {{- $additionalControllerVolumeMounts | toYaml | nindent 12 }}
          {{- end }}
        {{- if $webhookEnabled }}
        - name: webhooks
          {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true "seccompProfile" true) | nindent 10 }}
          {{- if $webhooksCommand }}
          command:
            {{- $webhooksCommand | toYaml | nindent 12 }}
          {{- else }}
          command:
            - /webhooks
            - -tls-cert-file={{ $webhooksCertMountPath }}/tls.crt
            - -tls-key-file={{ $webhooksCertMountPath }}/tls.key
          {{- end }}
          image: {{ include "helm_lib_module_image" (list $context $webhooksImageName) }}
          imagePullPolicy: IfNotPresent
          volumeMounts:
            - name: webhook-certs
              mountPath: {{ $webhooksCertMountPath }}
              readOnly: true
          readinessProbe:
            httpGet:
              path: /healthz
              port: {{ $webhooksPort }}
              scheme: HTTPS
            initialDelaySeconds: 5
            failureThreshold: 2
            periodSeconds: 1
          livenessProbe:
            httpGet:
              path: /healthz
              port: {{ $webhooksPort }}
              scheme: HTTPS
            periodSeconds: 1
            failureThreshold: 3
          ports:
            - name: https
              containerPort: {{ $webhooksPort }}
              protocol: TCP
          resources:
            requests:
              {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 14 }}
{{- if not ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
              {{- include "helm_lib_module_webhooks_resources" $context | nindent 14 }}
{{- end }}
        {{- end }}
        {{- if $additionalContainers }}
        {{- $additionalContainers | toYaml | nindent 8 }}
        {{- end }}
      {{- if or $webhookEnabled $additionalVolumes }}
      volumes:
        {{- if $webhookEnabled }}
        - name: webhook-certs
          secret:
            secretName: webhooks-https-certs
        {{- end }}
        {{- if $additionalVolumes }}
        {{- $additionalVolumes | toYaml | nindent 8 }}
        {{- end }}
      {{- end }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_module_controller_log_level" (list . "csiHpe") }} */ -}}
{{- /* Returns numeric log level from module values */ -}}
{{- define "helm_lib_module_controller_log_level" }}
  {{- $context := index . 0 }}
  {{- $valuesKey := index . 1 }}
  {{- $moduleValues := index $context.Values $valuesKey }}
  {{- $logLevel := $moduleValues.logLevel | default "INFO" }}
  {{- if eq $logLevel "ERROR" -}}
0
  {{- else if eq $logLevel "WARN" -}}
1
  {{- else if eq $logLevel "INFO" -}}
2
  {{- else if eq $logLevel "DEBUG" -}}
3
  {{- else if eq $logLevel "TRACE" -}}
4
  {{- else -}}
2
  {{- end -}}
{{- end }}


{{- /* Usage: {{ include "helm_lib_module_webhook_service" (list . $config) }} */ -}}
{{- /*
  Generates webhook Service manifest.

  $config parameters:
  - fullname: name of the service (default: "webhooks")
  - selectorApp: app label for selector (default: "controller")
  - targetPort: target port name or number (default: "https")
  - additionalPorts: list of additional ports (optional), each port is a dict with:
    - name: port name (required)
    - port: service port (required)
    - targetPort: target port name or number (required)
    - protocol: protocol (default: "TCP")
*/ -}}
{{- define "helm_lib_module_webhook_service" }}
  {{- $context := index . 0 }}
  {{- $config := index . 1 | default dict }}
  
  {{- $fullname := $config.fullname | default "webhooks" }}
  {{- $selectorApp := $config.selectorApp | default "controller" }}
  {{- $targetPort := $config.targetPort | default "https" }}
  {{- $additionalPorts := $config.additionalPorts }}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $selectorApp)) | nindent 2 }}
spec:
  type: ClusterIP
  ports:
    - port: 443
      targetPort: {{ $targetPort }}
      protocol: TCP
      name: https
    {{- range $additionalPorts }}
    - name: {{ .name }}
      port: {{ .port }}
      targetPort: {{ .targetPort }}
      protocol: {{ .protocol | default "TCP" }}
    {{- end }}
  selector:
    app: {{ $selectorApp }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_module_validating_webhook_configuration" (list . $config) }} */ -}}
{{- /*
  Generates ValidatingWebhookConfiguration manifest.

  $config parameters:
  - name: webhook configuration name (required, e.g., "sc-validation")
  - webhookName: webhook name suffix (required, e.g., "sc-validation")
  - valuesKey: key to access module values (required, e.g., "csiHpe")
  - webhookCertPath: path to webhook cert in values (default: "internal.customWebhookCert")
  - serviceName: service name (default: "webhooks")
  - path: webhook path (required, e.g., "/sc-validate")
  - rules: webhook rules (required)
  - matchConditions: optional match conditions
  - sideEffects: side effects (default: "None")
  - timeoutSeconds: timeout (default: 5)
*/ -}}
{{- define "helm_lib_module_validating_webhook_configuration" }}
  {{- $context := index . 0 }}
  {{- $config := index . 1 }}
  
  {{- $name := $config.name | required "$config.name is required" }}
  {{- $webhookName := $config.webhookName | required "$config.webhookName is required" }}
  {{- $valuesKey := $config.valuesKey | required "$config.valuesKey is required" }}
  {{- $webhookCertPath := $config.webhookCertPath | default "internal.customWebhookCert" }}
  {{- $serviceName := $config.serviceName | default "webhooks" }}
  {{- $path := $config.path | required "$config.path is required" }}
  {{- $rules := $config.rules | required "$config.rules is required" }}
  {{- $matchConditions := $config.matchConditions }}
  {{- $sideEffects := $config.sideEffects | default "None" }}
  {{- $timeoutSeconds := $config.timeoutSeconds | default 5 }}

  {{- /* Get module values and cert */ -}}
  {{- $moduleValues := index $context.Values $valuesKey }}
  {{- $certPath := printf "%s.ca" $webhookCertPath }}
  {{- $certValue := $moduleValues }}
  {{- range $part := (split "." $certPath) }}
    {{- $certValue = index $certValue $part }}
  {{- end }}
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: "d8-{{ $context.Chart.Name }}-{{ $name }}"
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
webhooks:
  - name: "d8-{{ $context.Chart.Name }}-{{ $webhookName }}.deckhouse.io"
    rules:
      {{- $rules | toYaml | nindent 6 }}
    clientConfig:
      service:
        namespace: "d8-{{ $context.Chart.Name }}"
        name: {{ $serviceName | quote }}
        path: {{ $path | quote }}
      caBundle: {{ $certValue | b64enc | quote }}
    admissionReviewVersions: ["v1", "v1beta1"]
    sideEffects: {{ $sideEffects }}
    timeoutSeconds: {{ $timeoutSeconds }}
    {{- if $matchConditions }}
    matchConditions:
      {{- $matchConditions | toYaml | nindent 6 }}
    {{- end }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_module_webhook_certs_secret" (list . $config) }} */ -}}
{{- /*
  Generates webhook TLS certificates Secret manifest.

  $config parameters:
  - fullname: name of the secret (default: "webhooks-https-certs")
  - valuesKey: key to access module values (required, e.g., "csiHpe", "csiS3")
  - webhookCertPath: path to webhook cert in values (default: "internal.customWebhookCert")
  - appLabel: app label for the secret (default: "webhooks")
*/ -}}
{{- define "helm_lib_module_webhook_certs_secret" }}
  {{- $context := index . 0 }}
  {{- $config := index . 1 }}
  
  {{- $fullname := $config.fullname | default "webhooks-https-certs" }}
  {{- $valuesKey := $config.valuesKey | required "$config.valuesKey is required" }}
  {{- $webhookCertPath := $config.webhookCertPath | default "internal.customWebhookCert" }}
  {{- $appLabel := $config.appLabel | default "webhooks" }}

  {{- /* Get module values and certs */ -}}
  {{- $moduleValues := index $context.Values $valuesKey }}
  {{- $certValue := $moduleValues }}
  {{- range $part := (split "." $webhookCertPath) }}
    {{- $certValue = index $certValue $part }}
  {{- end }}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $appLabel)) | nindent 2 }}
type: kubernetes.io/tls
data:
  ca.crt: {{ $certValue.ca | b64enc | quote }}
  tls.crt: {{ $certValue.crt | b64enc | quote }}
  tls.key: {{ $certValue.key | b64enc | quote }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_module_controller_rbac" (list . $config) }} */ -}}
{{- /*
  Generates controller RBAC manifests (ServiceAccount, Role, ClusterRole, RoleBinding, ClusterRoleBinding).

  $config parameters:
  - fullname: name for the resources (default: "controller")
  - roleRules: rules for namespaced Role (required)
  - clusterRoleRules: rules for ClusterRole (required)
*/ -}}
{{- define "helm_lib_module_controller_rbac" -}}
  {{- $context := index . 0 -}}
  {{- $config := index . 1 -}}
  {{- $fullname := $config.fullname | default "controller" -}}
  {{- $roleRules := $config.roleRules | required "$config.roleRules is required" -}}
  {{- $clusterRoleRules := $config.clusterRoleRules | required "$config.clusterRoleRules is required" }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $fullname)) | nindent 2 }}
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $fullname)) | nindent 2 }}
rules:
  {{- $roleRules | toYaml | nindent 2 }}
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: d8:{{ $context.Chart.Name }}:{{ $fullname }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $fullname)) | nindent 2 }}
rules:
  {{- $clusterRoleRules | toYaml | nindent 2 }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ $fullname }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $fullname)) | nindent 2 }}
subjects:
  - kind: ServiceAccount
    name: {{ $fullname }}
    namespace: d8-{{ $context.Chart.Name }}
roleRef:
  kind: Role
  name: {{ $fullname }}
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:{{ $context.Chart.Name }}:{{ $fullname }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" $fullname)) | nindent 2 }}
subjects:
  - kind: ServiceAccount
    name: {{ $fullname }}
    namespace: d8-{{ $context.Chart.Name }}
roleRef:
  kind: ClusterRole
  name: d8:{{ $context.Chart.Name }}:{{ $fullname }}
  apiGroup: rbac.authorization.k8s.io
{{- end }}



