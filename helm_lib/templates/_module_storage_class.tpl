{{- /* Usage: {{ include "helm_lib_module_storage_class_annotations" (list $ $index $storageClass.name) }} */ -}}
{{- /* return module StorageClass annotations */ -}}
{{- define "helm_lib_module_storage_class_annotations" -}}
  {{- $context := index . 0 -}}
  {{- $sc_index := index . 1  -}}
  {{- $sc_name := index . 2  -}}
  {{- $module_values := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) -}}
  {{- $annotations := dict -}}

  {{- $volume_expansion_mode_offline := false -}}
  {{- range $module_name := list "cloud-provider-azure" "cloud-provider-yandex" "cloud-provider-vsphere" }}
    {{- if has $module_name $context.Values.global.enabledModules }}
      {{- $volume_expansion_mode_offline = true }}
    {{- end }}
  {{- end }}

  {{- if $volume_expansion_mode_offline }}
    {{- $_ := set $annotations "storageclass.deckhouse.io/volume-expansion-mode" "offline" }}
  {{- end }}

  {{- if hasKey $module_values.internal "defaultStorageClass" }}
    {{- if eq $module_values.internal.defaultStorageClass $sc_name }}
      {{- $_ := set $annotations "storageclass.kubernetes.io/is-default-class" "true" }}
    {{- end }}
  {{- else }}
    {{- if eq $sc_index 0 }}
      {{- if $context.Values.global.discovery.defaultStorageClass }}
        {{- if eq $context.Values.global.discovery.defaultStorageClass $sc_name }}
          {{- $_ := set $annotations "storageclass.kubernetes.io/is-default-class" "true" }}
        {{- end }}
      {{- else }}
        {{- $_ := set $annotations "storageclass.kubernetes.io/is-default-class" "true" }}
      {{- end }}
    {{- end }}
  {{- end }}

{{- (dict "annotations" $annotations) | toYaml -}}
{{- end -}}
