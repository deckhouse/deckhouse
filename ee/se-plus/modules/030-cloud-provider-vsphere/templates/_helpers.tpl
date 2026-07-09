{{- define "vsphere_has_nsxt_caBundle" -}}
{{- if and .Values.cloudProviderVsphere.internal.providerClusterConfiguration.nsxt .Values.cloudProviderVsphere.internal.providerClusterConfiguration.nsxt.caBundle -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{- define "vsphere_has_provider_caBundle" -}}
{{- if and .Values.cloudProviderVsphere.internal.providerClusterConfiguration.provider .Values.cloudProviderVsphere.internal.providerClusterConfiguration.provider.caBundle -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{- define "vsphere_has_caBundle" -}}
{{- if or (eq (include "vsphere_has_provider_caBundle" .) "true") (eq (include "vsphere_has_nsxt_caBundle" .) "true") -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}
