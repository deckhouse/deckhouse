{{/* 
  This template defines the minimum resource requests for the registry_incluster_proxy.
*/}}
{{- define "min_resources_for_registry_incluster_proxy" }}
cpu: 25m
memory: 40Mi
{{- end }}

{{/* 
  This template defines the maximum resource limits for the registry_incluster_proxy.
*/}}
{{- define "max_resources_for_registry_incluster_proxy" }}
cpu: 50m
memory: 50Mi
{{- end }}
