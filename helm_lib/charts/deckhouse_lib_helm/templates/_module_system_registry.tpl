{{/*
isRegistryEnabled checks if the system registry module is enabled.

Example:
{{- if (include "isRegistryEnabled" $) }}
  # Actions to take if the system registry module is enabled
{{- else }}
  # Actions to take if the system registry module is not enabled
{{- end }}

Parameters:
- `$` : The current context, which includes values defined in values.yaml and other template context.

Returns:
- true/false if the system registry module enabled/disabled.
*/}}
{{- define "isRegistryEnabled" -}}
{{- $enabled := false -}}  {{/* Initialize the variable as false */}}
{{- if (.Values.global.enabledModules | has "registry") }}
  {{- $enabled = true -}}
{{- end -}}
{{- $enabled -}}  {{/* Return the value of the variable */}}
{{- end -}}
