{{- define "pod_security_standard_baseline" }}
  {{- $context := index . 0 }}
  {{- $policyCRDName := index . 1 }}
  {{- $parameters := dict }}
  {{- if gt (len .) 2 }}
  {{- $parameters = index . 2}}
  {{- end}}

{{- include "pod_security_standard_base" (list $context "baseline" $policyCRDName $parameters ) }}
{{- end }}

{{- define "pod_security_standard_restricted" }}
  {{- $context := index . 0 }}
  {{- $policyCRDName := index . 1 }}
  {{- $parameters := dict }}
  {{- if gt (len .) 2 }}
  {{- $parameters = index . 2}}
  {{- end}}

{{- include "pod_security_standard_base" (list $context "restricted" $policyCRDName $parameters ) }}
{{- end }}

{{- define "pod_security_standard_base" }}
  {{- $context := index . 0 }}
  {{- $standard := index . 1 }}
  {{- $policyCRDName := index . 2 }}
  {{- $parameters := index . 3 }}

{{- if $context.Values.admissionPolicyEngine.internal.bootstrapped }}
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: {{ $policyCRDName }}
metadata:
  name: d8-pod-security-{{$standard}}
  {{- include "helm_lib_module_labels" (list $context (dict "security.deckhouse.io/pod-standard" $standard)) | nindent 2 }}
spec:
  enforcementAction: {{ $context.Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower }}
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
