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
      - apiGroups: [""]
        kinds: ["Pod"]
    labelSelector:
      matchExpressions:
        - key: security.deckhouse.io/skip-pss-check
          operator: NotIn
          values: ["true"]
    namespaceSelector:
      matchExpressions:
      {{- if eq $standard "baseline" }}
        {{- if eq $defaultPolicy "privileged" }}
        - { key: security.deckhouse.io/pod-policy, operator: In, values: [ baseline, restricted ] }
        {{- else }}
        - { key: security.deckhouse.io/pod-policy, operator: NotIn, values: [ privileged ] }
        {{- end }}
      {{- else if eq $standard "restricted" }}
        {{- if eq $defaultPolicy "restricted" }}
        - { key: security.deckhouse.io/pod-policy, operator: NotIn, values: [ privileged, baseline ] }
        {{- else }}
        - { key: security.deckhouse.io/pod-policy, operator: In, values: [ restricted ] }
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
{{- end }}
{{- end }}

{{- define "trivy.provider.enabled" }}
  {{- $context := . }}
  {{- if and ($context.Values.global.enabledModules | has "operator-trivy") ($context.Values.admissionPolicyEngine.denyVulnerableImages.enabled) }}
    {{- print "true" }}
  {{- end }}
  {{- print "" }}
{{- end }}
