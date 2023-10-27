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

{{- if $context.Values.admissionPolicyEngine.internal.bootstrapped }}
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: {{ $policyCRDName }}
metadata:
  name: d8-pod-security-{{$standard}}-{{$policyAction}}
  {{- include "helm_lib_module_labels" (list $context (dict "security.deckhouse.io/pod-standard" $standard)) | nindent 2 }}
spec:
  enforcementAction: {{ $policyAction }}
  match:
    scope: Namespaced
    kinds:
      - apiGroups: [""]
        kinds: ["Pod"]
    namespaceSelector:
      {{- if eq $standard "baseline" }}
      matchExpressions:
        - { key: security.deckhouse.io/pod-policy, operator: In, values: [ baseline,restricted ] }
      {{- else if eq $standard "restricted" }}
      matchLabels:
        security.deckhouse.io/pod-policy: restricted
      {{- else}}
        {{ cat "Unknown policy standard" | fail }}
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
