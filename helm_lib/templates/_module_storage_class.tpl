{{- /* Usage: {{ include "helm_lib_module_storage_class_annotations" (list $ $index $storageClass.name) }} */ -}}
{{- /* return module StorageClass annotations */ -}}
{{- define "helm_lib_module_storage_class_annotations" -}}
  {{- $context := index . 0 -}}
  {{- $sc_index := index . 1  -}}
  {{- $sc_name := index . 2  -}}
  {{- $module_values := include "helm_lib_module_values" $context | fromYaml -}}

  {{- if hasKey $module_values.internal "defaultStorageClass" }}
    {{- if eq $module_values.internal.defaultStorageClass $sc_name }}
annotations:
  storageclass.kubernetes.io/is-default-class: "true"
    {{- end }}
  {{- else }}
    {{- if eq $sc_index 0 }}
      {{- if $context.Values.global.discovery.defaultStorageClass }}
        {{- if eq $context.Values.global.discovery.defaultStorageClass $sc_name }}
annotations:
  storageclass.kubernetes.io/is-default-class: "true"
        {{- end }}
      {{- else }}
annotations:
  storageclass.kubernetes.io/is-default-class: "true"
      {{- end }}
    {{- end }}
  {{- end }}
{{- end -}}
