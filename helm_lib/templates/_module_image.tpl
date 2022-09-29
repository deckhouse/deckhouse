{{- /* Usage: {{ include "helm_lib_module_image" (list . "<container-name>") }} */ -}}
{{- /* returns image name */ -}}
{{- define "helm_lib_module_image" }}
  {{- $context := index . 0 }}
  {{- $containerName := index . 1 | trimAll "\"" }}
  {{- $moduleName := $context.Chart.Name | replace "-" "_" | camelcase | untitle }}
  {{- $imageHash := index $context.Values.global.modulesImages.tags $moduleName $containerName }}
  {{- if not $imageHash }}
  {{- $error := (printf "Image %s.%s has no tag" $moduleName $containerName ) }}
  {{- fail $error }}
  {{- end }}
  {{- printf "%s:%s" $context.Values.global.modulesImages.registry $imageHash }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_common_image" (list . "<container-name>") }} */ -}}
{{- /* returns image name from common module */ -}}
{{- define "helm_lib_module_common_image" }}
  {{- $context := index . 0 }}
  {{- $containerName := index . 1 | trimAll "\"" }}
  {{- $imageHash := index $context.Values.global.modulesImages.tags "common" $containerName }}
  {{- if not $imageHash }}
  {{- $error := (printf "Image %s.%s has no tag" "common" $containerName ) }}
  {{- fail $error }}
  {{- end }}
  {{- printf "%s:%s" $context.Values.global.modulesImages.registry $imageHash }}
{{- end }}
