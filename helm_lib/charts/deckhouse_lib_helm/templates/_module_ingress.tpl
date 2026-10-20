{{- /* Usage: {{ include "helm_lib_module_ingress_class" . }} */ -}}
{{- /* returns ingress class from module settings or if not exists from global config */ -}}
{{- define "helm_lib_module_ingress_class" -}}
  {{- $context := . -}}
  {{- $moduleValues := index $context.Values (include "helm_lib_module_camelcase_name" $context) -}}

  {{- if and
        (hasKey $moduleValues "ingress")
        (hasKey $moduleValues.ingress "ingressClass")
  -}}
    {{- $moduleValues.ingress.ingressClass -}}
  {{- else if hasKey $moduleValues "ingressClass" -}}
    {{- /* Deprecated module schema. */ -}}
    {{- $moduleValues.ingressClass -}}
  {{- else if and
        (hasKey $context.Values.global.modules "ingress")
        (hasKey $context.Values.global.modules.ingress "ingressClass")
  -}}
    {{- $context.Values.global.modules.ingress.ingressClass -}}
  {{- else if hasKey $context.Values.global.modules "ingressClass" -}}
    {{- /* Deprecated global schema. */ -}}
    {{- $context.Values.global.modules.ingressClass -}}
  {{- end -}}
{{- end -}}

{{- /* Usage: nginx.ingress.kubernetes.io/configuration-snippet: | {{ include "helm_lib_module_ingress_configuration_snippet" . | nindent 6 }} */ -}}
{{- /* returns nginx ingress additional headers (e.g. HSTS) if HTTPS is enabled */ -}}
{{- define "helm_lib_module_ingress_configuration_snippet" -}}
  {{- $context := . -}} {{- /* Template context with .Values, .Chart, etc */ -}}

  {{- $mode := include "helm_lib_module_https_mode" $context -}}

  {{- if or (eq "CertManager" $mode) (eq "CustomCertificate" $mode) -}}
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
  {{- end -}}
{{- end -}}

{{- /* Usage: {{- if eq (include "helm_lib_module_ingress_enabled" .) "true" }} */ -}}
{{- /* returns whether Ingress is enabled from module settings or if not exists from global config */ -}}
{{- define "helm_lib_module_ingress_enabled" -}}
  {{- $context := . -}}
  {{- $moduleValues := index $context.Values (include "helm_lib_module_camelcase_name" $context) -}}

  {{- if and
        (hasKey $moduleValues "ingress")
        (hasKey $moduleValues.ingress "enabled")
  -}}
    {{- $moduleValues.ingress.enabled -}}
  {{- else if and
        (hasKey $context.Values.global.modules "ingress")
        (hasKey $context.Values.global.modules.ingress "enabled")
  -}}
    {{- $context.Values.global.modules.ingress.enabled -}}
  {{- else -}}
    true
  {{- end -}}
{{- end -}}

{{- /* Usage: {{- if include "helm_lib_module_ingress_nginx_annotations_version_ge" (list . "1.3.0") }} */ -}}
{{- /* returns "true" if the generation of the additional Ingress annotations the ingress-nginx controllers support, discovered in .Values.global.discovery.ingressNginxAnnotationsVersion, is a semver greater than or equal to the given one */ -}}
{{- /* returns an empty string if the discovered version is missing, empty or not a semver, so the caller falls back */ -}}
{{- define "helm_lib_module_ingress_nginx_annotations_version_ge" -}}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $minimal := index . 1 | toString -}} {{- /* Minimal version the discovered one must reach, e.g. "1.3.0" or "v1.3.0" */ -}}

  {{- $semver := "^v?[0-9]+(\\.[0-9]+)?(\\.[0-9]+)?(-[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?(\\+[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$" -}}
  {{- if not (regexMatch $semver $minimal) -}}
    {{- fail (printf "helm_lib_module_ingress_nginx_annotations_version_ge: %q is not a semver" $minimal) -}}
  {{- end -}}

  {{- $discovered := "" -}}
  {{- $global := $context.Values.global -}}
  {{- if and (kindIs "map" $global) (kindIs "map" $global.discovery) -}}
    {{- $discovered = $global.discovery.ingressNginxAnnotationsVersion | default "" | toString -}}
  {{- end -}}
  {{- if and (regexMatch $semver $discovered) (semverCompare (printf ">=%s" $minimal) $discovered) -}}
    true
  {{- end -}}
{{- end -}}
