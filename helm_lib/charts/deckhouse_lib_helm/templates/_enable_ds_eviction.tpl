{{- /* Usage: {{ include "helm_lib_prevent_ds_eviction_annotation" . }} */ -}}
{{- /* Adds `cluster-autoscaler.kubernetes.io/enable-ds-eviction` annotation to manage DaemonSet eviction by the Cluster Autoscaler. */ -}}
{{- /* This is important to prevent the eviction of DaemonSet pods during cluster scaling.  */ -}}
{{- define "helm_lib_prevent_ds_eviction_annotation" -}}
cluster-autoscaler.kubernetes.io/enable-ds-eviction: "false"
{{- end }}
