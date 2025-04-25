{{- define "min_resources_for_registry_incluster_proxy" }}
cpu: 25m
memory: 40Mi
{{- end }}

{{- define "max_resources_for_registry_incluster_proxy" }}
cpu: 50m
memory: 50Mi
{{- end }}