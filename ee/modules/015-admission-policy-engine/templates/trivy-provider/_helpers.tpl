{{- define "trivy.provider.enabled" }}
  {{- $context := . }}
  {{- if and ($context.Values.global.enabledModules | has "operator-trivy") ($context.Values.admissionPolicyEngine.trivyProvider.enable) }}
    {{- print "true" }}
  {{- end }}
  {{- print "" }}
{{- end }}
