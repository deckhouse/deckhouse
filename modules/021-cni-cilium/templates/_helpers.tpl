{{- /* ClusterMesh comes with the editions that have the peering module, EE and FE: elsewhere the setting changes nothing. */ -}}
{{- define "cni_cilium_clustermesh_enabled" -}}
{{- if and (dig "clusterMesh" "enabled" false .Values.cniCilium) (has (dig "deckhouseEdition" "" .Values.global) (list "EE" "FE")) -}}
true
{{- end -}}
{{- end -}}

{{- /* The cluster name in the mesh: the cluster UUID without dashes, 32 characters, the most Cilium allows */ -}}
{{- define "clustermesh_cluster_name" -}}
{{- dig "discovery" "clusterUUID" "" .Values.global | replace "-" "" | lower -}}
{{- end -}}
