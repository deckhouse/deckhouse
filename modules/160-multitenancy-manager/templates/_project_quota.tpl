{{- /* "true" where Deckhouse manages the control plane of the cluster: the sum of a project quota is enforced only there. */ -}}
{{- define "multitenancy_manager_control_plane_managed" -}}
{{- has "control-plane-manager" .Values.global.enabledModules | toString -}}
{{- end -}}

{{- /* The families of objects the quota ledger counts besides the ResourceQuota objects: the ones whose module is enabled, comma-separated. */ -}}
{{- define "multitenancy_manager_quota_ledger_families" -}}
{{- $families := list -}}
{{- if has "virtualization" .Values.global.enabledModules -}}
{{- $families = append $families "vm" -}}
{{- end -}}
{{- join "," $families -}}
{{- end -}}

{{- /*
  The intervals of the quota ledger. The ledger refuses to start when lease - fence - webhook timeout is
  below 9 seconds, and the webhook of the ledger takes its timeoutSeconds from here, so the two cannot
  disagree.
*/ -}}
{{- define "multitenancy_manager_quota_ledger_webhook_timeout" -}}
1
{{- end -}}
{{- define "multitenancy_manager_quota_ledger_timing" -}}
leaseSeconds: "17"
renewDeadlineSeconds: "10"
retryPeriodSeconds: "1"
fenceSeconds: "7"
webhookTimeoutSeconds: {{ include "multitenancy_manager_quota_ledger_webhook_timeout" . | quote }}
{{- end -}}

{{- /*
  What both guards of a quota namespace look at, the pending one and the enforced one, so that the two
  cannot drift apart. The objects that cannot wait for the quota are left out by excludeResourceRules:
  the API server checks the match constraints before it looks the parameter of a binding up, so the
  quota itself, events, reviews and the kube-root-ca.crt ConfigMap and the default ServiceAccount of a
  new namespace pass even while the d8-project-quota parameter of the enforced guard is missing. The
  writers are left out by matchConditions, which the API server evaluates only once the parameter is
  there.
*/ -}}
{{- define "multitenancy_manager_quota_guard_match" -}}
matchConstraints:
  resourceRules:
    - apiGroups:   ["*"]
      apiVersions: ["*"]
      operations:  ["CREATE"]
      resources:   ["*"]
      scope: Namespaced
    # a resize of a pod or of a volume claim raises what the quota counts
    - apiGroups:   [""]
      apiVersions: ["v1"]
      operations:  ["UPDATE"]
      resources:   ["pods/resize", "persistentvolumeclaims"]
      scope: Namespaced
  excludeResourceRules:
    - apiGroups:   [""]
      apiVersions: ["*"]
      operations:  ["*"]
      resources:   ["resourcequotas", "events"]
      scope: Namespaced
    - apiGroups:   ["events.k8s.io"]
      apiVersions: ["*"]
      operations:  ["*"]
      resources:   ["events"]
      scope: Namespaced
    - apiGroups:   ["authorization.k8s.io"]
      apiVersions: ["*"]
      operations:  ["*"]
      resources:   ["localsubjectaccessreviews"]
      scope: Namespaced
    - apiGroups:     [""]
      apiVersions:   ["*"]
      operations:    ["*"]
      resources:     ["configmaps"]
      resourceNames: ["kube-root-ca.crt"]
      scope: Namespaced
    - apiGroups:     [""]
      apiVersions:   ["*"]
      operations:    ["*"]
      resources:     ["serviceaccounts"]
      resourceNames: ["default"]
      scope: Namespaced
matchConditions:
  - name: 'exclude-the-quota-and-reviews'
    expression: '!(request.resource.group == "" && request.resource.resource in ["resourcequotas", "events"]) && request.resource.group != "events.k8s.io" && !request.resource.resource.endsWith("reviews")'
  - name: 'exclude-the-objects-of-a-new-namespace'
    expression: '!(request.resource.group == "" && ((request.resource.resource == "configmaps" && request.name == "kube-root-ca.crt") || (request.resource.resource == "serviceaccounts" && request.name == "default")))'
  - name: 'exclude-writers'
    expression: '!(request.userInfo.username in ["system:serviceaccount:d8-multitenancy-manager:multitenancy-manager", "system:serviceaccount:d8-system:deckhouse", "system:sudouser"])'
{{- end -}}
