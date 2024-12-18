{{- /* Usage: {{ include "helm_lib_kind_exists" (list . "<kind-name>") }} */ -}}
{{- /* returns true if the specified resource kind (case-insensitive) is represented in the cluster */ -}}
{{- define "helm_lib_kind_exists" }}
  {{- $context      := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $kind_name := index . 1 -}} {{- /* Kind name portion */ -}}
  {{- if eq (len $context.Capabilities.APIVersions) 0 -}}
    {{- fail "Helm reports no capabilities" -}}
  {{- end -}}
  {{ range $cap := $context.Capabilities.APIVersions }}
    {{- if hasSuffix (lower (printf "/%s" $kind_name)) (lower $cap) }}
      found
      {{- break }}
    {{- end }}
  {{- end }}
{{- end -}}

{{- /* Usage: {{ include "helm_lib_get_api_version_by_kind" (list . "<kind-name>") }} */ -}}
{{- /* returns current apiVersion string, based on available helm capabilities, for the provided kind (not all kinds are supported) */ -}}
{{- define "helm_lib_get_api_version_by_kind" }}
  {{- $context      := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $kind_name := index . 1 -}} {{- /* Kind name portion */ -}}
  {{- if eq (len $context.Capabilities.APIVersions) 0 -}}
    {{- fail "Helm reports no capabilities" -}}
  {{- end -}}
  {{- if or (eq $kind_name "ValidatingAdmissionPolicy") (eq $kind_name "ValidatingAdmissionPolicyBinding") -}}
    {{- if $context.Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy" -}}
admissionregistration.k8s.io/v1
    {{- else if $context.Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1beta1/ValidatingAdmissionPolicy" -}}
admissionregistration.k8s.io/v1beta1
    {{- else -}}
admissionregistration.k8s.io/v1alpha1
    {{- end -}}
  {{- else -}}
    {{- fail (printf "Kind '%s' isn't supported by the 'helm_lib_get_api_version_by_kind' helper" $kind_name) -}}
  {{- end -}}
{{- end -}}
