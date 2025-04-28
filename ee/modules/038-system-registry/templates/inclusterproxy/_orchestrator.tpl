{{/* 
  This template retrieves the hash sum of the orchestrator configuration.
  
  Example:
  {{- $ctx := .}}
  orchestratorHash: {{ include "orchestrator_hash" $ctx }}
*/}}
{{- define "orchestrator_hash" -}}
{{- $ctx := . -}}
{{- $hash := "" -}}
{{- with $ctx.Values.systemRegistry.internal.orchestrator -}}
    {{- with .hash -}}
        {{- $hash = . -}}
    {{- end -}}
{{- end -}}
{{- $hash -}}
{{- end -}}