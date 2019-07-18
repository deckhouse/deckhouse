{{- /* Usage: {{ include "helm_lib_addon_public_domain" (list . "<name-portion>") }} */ -}}
{{- /* returns rendered addonsPublicDomainTemplate to service fqdn */ -}}
{{- define "helm_lib_addon_public_domain" }}
  {{- $context      := index . 0 -}} {{- /* Dot object (.) with .Values, .Chart, etc */ -}}
  {{- $name_portion := index . 1 -}} {{- /* argv1 */ -}}

  {{- if not (contains "%s" $context.Values.global.addonsPublicDomainTemplate) }}
    {{ fail "Error!!! global.addonsPublicDomainTemplate must contain \"%s\" pattern to render service fqdn!" }}
  {{- end }}
  {{- printf $context.Values.global.addonsPublicDomainTemplate $name_portion }}
{{- end }}
