{{- /* Usage: {{ include "helm_lib_module_public_domain" (list . "<name-portion>") }} */ -}}
{{- /* returns rendered publicDomainTemplate to service fqdn */ -}}
{{- define "helm_lib_module_public_domain" }}
  {{- $context      := index . 0 -}} {{- /* Dot object (.) with .Values, .Chart, etc */ -}}
  {{- $name_portion := index . 1 -}} {{- /* argv1 */ -}}

  {{- if not (contains "%s" $context.Values.global.modules.publicDomainTemplate) }}
    {{ fail "Error!!! global.modules.publicDomainTemplate must contain \"%s\" pattern to render service fqdn!" }}
  {{- end }}
  {{- printf $context.Values.global.modules.publicDomainTemplate $name_portion }}
{{- end }}
