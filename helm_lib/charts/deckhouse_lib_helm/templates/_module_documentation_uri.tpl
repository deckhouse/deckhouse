{{- /* Usage: {{ include "helm_lib_module_documentation_uri" (list . "<path_to_document>") }} */ -}}
{{- /* returns rendered documentation uri using publicDomainTemplate or deckhouse.io domains*/ -}}
{{- define "helm_lib_module_documentation_uri" }}
  {{- $default_doc_prefix := "https://deckhouse.io/documentation/v1" -}}
  {{- $context      := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $path_portion := index . 1 -}} {{- /* Path to the document */ -}}
  {{- $uri := "" -}}
  {{- if $context.Values.global.modules.publicDomainTemplate }}
     {{- $uri = printf "%s://%s%s" (include "helm_lib_module_uri_scheme" $context) (include "helm_lib_module_public_domain" (list $context "documentation")) $path_portion -}}
  {{- else }}
     {{- $uri = printf "%s%s" $default_doc_prefix $path_portion -}}
  {{- end -}}

  {{ $uri }}
{{- end }}
