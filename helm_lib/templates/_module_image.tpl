{{- /* Usage: {{ include "helm_lib_module_image" (list . "<container-name>") }} */ -}}
{{- /* returns image name */ -}}
{{- define "helm_lib_module_image" }}
  {{- $context := index . 0 }}
  {{- $containerName := index . 1 }}
  {{- $moduleName := $context.Chart.Name | replace "-" "_" | camelcase | untitle }}

  {{- /* imageHash default value for tests is "imageHash" */ -}}
  {{- $imageHash := index $context.Values.global.modulesImages.tags $moduleName $containerName | default "imageHash" }}

  {{- printf "%s:%s" $context.Values.global.modulesImages.registry $imageHash }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_common_image" (list . "<container-name>") }} */ -}}
{{- /* returns image name from common module */ -}}
{{- define "helm_lib_module_common_image" }}
  {{- $context := index . 0 }}
  {{- $containerName := index . 1 }}

  {{- /* imageHash default value for tests is "imageHash" */ -}}
  {{- $imageHash := index $context.Values.global.modulesImages.tags "common" $containerName | default "imageHash" }}

  {{- printf "%s:%s" $context.Values.global.modulesImages.registry $imageHash }}
{{- end }}
