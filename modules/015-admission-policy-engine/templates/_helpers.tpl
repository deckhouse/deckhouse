{{/*
  constraint_selector renders the namespace/label selectors from the CR's match section.
  NOTE on labelSelector semantics (PR #21556 review M3):
  Gatekeeper's match.labelSelector evaluates against the reviewed object's own
  metadata.labels. For Pods this is the pod's labels. For controllers (Deployment,
  StatefulSet, etc.) this is the controller's TOP-LEVEL metadata.labels, NOT the
  pod template's metadata.labels (spec.template.metadata.labels). Users who
  write labelSelector thinking about pod labels should be aware that a positive
  selector not mirrored at the controller top level will result in no controller-level
  check at all, and an exclusion-style selector (NotIn/DoesNotExist) used to exempt
  a workload will stop exempting at the controller level.
*/}}
{{- define "constraint_selector" }}
    {{- $cr := index . 0 }}

    {{- if $cr.spec.match.namespaceSelector }}
      {{- if hasKey $cr.spec.match.namespaceSelector "matchNames"}}
    namespaces:
      {{- $cr.spec.match.namespaceSelector.matchNames | toYaml | nindent 6 }}
      {{- end }}
      {{- if hasKey $cr.spec.match.namespaceSelector "excludeNames" }}
    excludedNamespaces:
      {{- $cr.spec.match.namespaceSelector.excludeNames | toYaml | nindent 6 }}
      {{- end }}
      {{- if hasKey $cr.spec.match.namespaceSelector "labelSelector" }}
    namespaceSelector:
      {{- $cr.spec.match.namespaceSelector.labelSelector | toYaml | nindent 6 }}
      {{- end }}
    {{- end }}
    {{- if hasKey $cr.spec.match "labelSelector" }}
    labelSelector:
      {{- $cr.spec.match.labelSelector | toYaml | nindent 6 }}
    {{- end }}
{{- end }}

{{- define "pod_security_standard_baseline" }}
  {{- $context := index . 0 }}
  {{- $policyCRDName := index . 1 }}
  {{- $policyAction := index . 2 }}
  {{- $parameters := dict }}
  {{- if gt (len .) 3 }}
  {{- $parameters = index . 3}}
  {{- end}}

{{- include "pod_security_standard_base" (list $context "baseline" $policyCRDName $policyAction $parameters) }}
{{- end }}

{{- define "pod_security_standard_restricted" }}
  {{- $context := index . 0 }}
  {{- $policyCRDName := index . 1 }}
  {{- $policyAction := index . 2 }}
  {{- $parameters := dict }}
  {{- if gt (len .) 3 }}
  {{- $parameters = index . 3}}
  {{- end}}

{{- include "pod_security_standard_base" (list $context "restricted" $policyCRDName $policyAction $parameters) }}
{{- end }}

{{- define "pod_security_standard_base" }}
  {{- $context := index . 0 }}
  {{- $standard := index . 1 }}
  {{- $policyCRDName := index . 2 }}
  {{- $policyAction := index . 3 }}
  {{- $parameters := index . 4 }}
  {{- $defaultPolicy := ($context.Values.admissionPolicyEngine.podSecurityStandards.defaultPolicy | default "privileged" | lower) }}

{{- if $context.Values.admissionPolicyEngine.internal.bootstrapped }}
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: {{ $policyCRDName }}
metadata:
{{- if eq $policyAction ($context.Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower) }}
  name: d8-pod-security-{{$standard}}-{{$policyAction}}-default
{{- else }}
  name: d8-pod-security-{{$standard}}-{{$policyAction}}
{{- end }}
  {{- include "helm_lib_module_labels" (list $context (dict "security.deckhouse.io/pod-standard" $standard)) | nindent 2 }}
spec:
  enforcementAction: {{ $policyAction }}
  match:
    scope: Namespaced
    kinds:
{{- include "workload_kinds" . }}
    labelSelector:
      matchExpressions:
        - key: security.deckhouse.io/skip-pss-check
          operator: NotIn
          values: ["true"]
        - key: gatekeeper.sh/operation
          operator: NotIn
          values: ["webhook"]
    namespaceSelector:
      matchExpressions:
      {{- if eq $standard "baseline" }}
        {{- if eq $defaultPolicy "privileged" }}
        - { key: security.deckhouse.io/pod-policy, operator: In, values: [ baseline, restricted ] }
        {{- else }}
        - { key: security.deckhouse.io/pod-policy, operator: NotIn, values: [ privileged ] }
        - { key: heritage, operator: NotIn, values: [ deckhouse ] }
        {{- end }}
      {{- else if eq $standard "restricted" }}
        {{- if eq $defaultPolicy "restricted" }}
        - { key: security.deckhouse.io/pod-policy, operator: NotIn, values: [ privileged, baseline ] }
        - { key: heritage, operator: NotIn, values: [ deckhouse ] }
        {{- else }}
        - { key: security.deckhouse.io/pod-policy, operator: In, values: [ restricted ] }
        - { key: heritage, operator: NotIn, values: [ deckhouse ] }
        {{- end }}
      {{- else}}
        {{ cat "Unknown policy standard" | fail }}
      {{- end }}
      # matches default enforcement action
      {{- if eq $policyAction ($context.Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower) }}
        # if there are other policy actions apart from the default one, we add all of them to NotIn list, so that the namespaces with such labels aren't subject to the default policy
        {{- if gt (len $context.Values.admissionPolicyEngine.internal.podSecurityStandards.enforcementActions) 1 }}
        - { key: security.deckhouse.io/pod-policy-action, operator: NotIn, values: [{{ (without $context.Values.admissionPolicyEngine.internal.podSecurityStandards.enforcementActions $policyAction | join ",") }}] }
        {{- end }}
      # matches another action (non-default)
      {{- else }}
        - { key: security.deckhouse.io/pod-policy-action, operator: In, values: [{{ $policyAction }}] }
      {{- end }}
  {{- if $parameters }}
  parameters:
    {{ $parameters | toYaml | nindent 4 }}
  {{- end }}
{{/* #### TODO: Remove after full migration to securityPolicyExceptions in all modules */}}
{{- if or (and (eq $standard "baseline") (ne $defaultPolicy "privileged"))  (and (eq $standard "restricted") (ne $defaultPolicy "restricted")) }} 
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: {{ $policyCRDName }}
metadata:
{{- if eq $policyAction ($context.Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower) }}
  name: d8-pod-security-{{$standard}}-{{$policyAction}}-d8-default
{{- else }}
  name: d8-pod-security-{{$standard}}-{{$policyAction}}-d8
{{- end }}
  {{- include "helm_lib_module_labels" (list $context (dict "security.deckhouse.io/pod-standard" $standard)) | nindent 2 }}
spec:
  enforcementAction: {{ $policyAction }}
  match:
    scope: Namespaced
    kinds:
{{- include "workload_kinds" . }}
    labelSelector:
      matchExpressions:
        - key: security.deckhouse.io/skip-pss-check
          operator: NotIn
          values: ["true"]
        - key: gatekeeper.sh/operation
          operator: NotIn
          values: ["webhook"]
    namespaceSelector:
      matchExpressions:
    {{- if eq $standard "baseline" }}
      {{- if ne $defaultPolicy "privileged" }}
        - { key: heritage, operator: In, values: [ deckhouse ] }
        - { key: security.deckhouse.io/enable-security-policy-check, operator: In, values: [ "true" ] }
      {{- end }}
    {{- else if eq $standard "restricted" }}
      {{- if ne $defaultPolicy "restricted" }}
        - { key: heritage, operator: In, values: [ deckhouse ] }
        - { key: security.deckhouse.io/enable-security-policy-check, operator: In, values: [ "true" ] }
      {{- end }}
    {{- end }}
      # matches default enforcement action
      {{- if eq $policyAction ($context.Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower) }}
        # if there are other policy actions apart from the default one, we add all of them to NotIn list, so that the namespaces with such labels aren't subject to the default policy
        {{- if gt (len $context.Values.admissionPolicyEngine.internal.podSecurityStandards.enforcementActions) 1 }}
        - { key: security.deckhouse.io/pod-policy-action, operator: NotIn, values: [{{ (without $context.Values.admissionPolicyEngine.internal.podSecurityStandards.enforcementActions $policyAction | join ",") }}] }
        {{- end }}
      # matches another action (non-default)
      {{- else }}
        - { key: security.deckhouse.io/pod-policy-action, operator: In, values: [{{ $policyAction }}] }
      {{- end }}
  {{- if $parameters }}
  parameters:
    {{ $parameters | toYaml | nindent 4 }}
  {{- end }} 
{{- end }} 
{{/* #### end of TODO */}}
{{- end }}
{{- end }}

{{- define "trivy.provider.enabled" }}
  {{- $context := . }}
  {{- $denyEnabled := dig "operatorTrivy" "denyVulnerableImages" "enabled" false ($context.Values | merge (dict)) }}
  {{- if and ($context.Values.global.enabledModules | has "operator-trivy") $denyEnabled }}
    {{- print "true" }}
  {{- end }}
  {{- print "" }}
{{- end }}

{{/* workload_kinds outputs the standard Gatekeeper match.kinds blocks for pod-creating workloads. */}}
{{/* RS (replica set) is intentionally excluded — generated by Deployment, so a denial */}}
{{/* surfaces only in Deployment status and gives none of the early feedback the feature aims for. */}}
{{/* Usage: include "workload_kinds" . — indents with 6 spaces for match.kinds context. */}}
{{/* The first element of the passed list is the context, used to read controllerValidation. */}}
{{- define "workload_kinds" }}
  {{- $context := index . 0 }}
  {{- $controllerValidation := true }}
  {{- if hasKey $context.Values.admissionPolicyEngine "podSecurityStandards" }}
    {{- if hasKey $context.Values.admissionPolicyEngine.podSecurityStandards "controllerValidation" }}
      {{- $controllerValidation = $context.Values.admissionPolicyEngine.podSecurityStandards.controllerValidation }}
    {{- end }}
  {{- end }}
      - apiGroups: [""]
        kinds: ["Pod"]
  {{- if $controllerValidation }}
      - apiGroups: [apps]
        kinds: [Deployment, StatefulSet, DaemonSet]
      - apiGroups: [""]
        kinds: [ReplicationController]
      - apiGroups: [batch]
        kinds: [Job, CronJob]
  {{- end }}
{{- end }}
