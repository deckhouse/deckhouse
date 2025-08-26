{{- define "prompp_context" -}}
{{- $values := deepCopy .Values | merge dict }}
{{- $_ := set $values.global.modulesImages.registry "base" (printf "%s/modules/prompp" .Values.global.modulesImages.registry.base) }}
{{- $ctx := dict "Chart" (dict "Name" "prompp") "Values" $values }}
{{- $ctx | toYaml }}
{{- end }}

{{- define "prometheus_init_containers" -}}
{{- $ctx := index . 0 }}
{{- $volume := index . 1 }}
{{- if hasKey $ctx.Values.global.modulesImages.digests "prompp" }}
initContainers:
- name: prompptool
  image: {{ include "helm_lib_module_image" (list (include "prompp_context" $ctx | fromYaml) "prompptool") }}
  command:
  - /bin/prompptool
  - "--working-dir=/prometheus"
  - "--verbose"
  {{- if ($ctx.Values.global.enabledModules | has "prompp") }}
  - "walvanilla"
  {{- else }}
  - "walpp"
  {{- end }}
  volumeMounts:
  - name: {{ $volume }}
    mountPath: /prometheus
    subPath: prometheus-db
  {{- include "helm_lib_module_container_security_context_read_only_root_filesystem" $ctx | nindent 2 }}
  resources:
    requests:
      {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 6 }}
{{- end }}
{{- end }}
