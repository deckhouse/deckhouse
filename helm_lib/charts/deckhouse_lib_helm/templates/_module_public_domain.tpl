{{- /* Usage: {{ include "helm_lib_module_public_domain" (list . "<name-portion>") }} */ -}}
{{- /* returns rendered publicDomainTemplate to service fqdn */ -}}
{{- define "helm_lib_module_public_domain" }}
  {{- $context      := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $name_portion := index . 1 -}} {{- /* Name portion */ -}}

  {{- if not (contains "%s" $context.Values.global.modules.publicDomainTemplate) }}
    {{ fail "Error!!! global.modules.publicDomainTemplate must contain \"%s\" pattern to render service fqdn!" }}
  {{- end }}

  {{- $domain := printf $context.Values.global.modules.publicDomainTemplate $name_portion -}}

  {{- $domain_length := len $domain -}}
  {{- if gt $domain_length 64 -}}
    {{ fail "domain name must be not longer than 64 characters" }}
  {{- end -}}

  {{ $domain }}
{{- end }}
