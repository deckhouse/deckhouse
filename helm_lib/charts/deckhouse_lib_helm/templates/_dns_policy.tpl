{{- /* Usage: {{ include "helm_lib_dns_policy_bootstraping_state" (list . "Default" "ClusterFirstWithHostNet") }} */ -}}
{{- /* returns the proper dnsPolicy value depending on the cluster bootstrap phase */ -}}
{{- define "helm_lib_dns_policy_bootstraping_state" }}
{{- $context := index . 0 }}
{{- $valueDuringBootstrap := index . 1 }}
{{- $valueAfterBootstrap := index . 2 }}
{{- if $context.Values.global.clusterIsBootstrapped }}
{{- printf $valueAfterBootstrap }}
{{- else }}
{{- printf $valueDuringBootstrap }}
{{- end }}
{{- end }}
