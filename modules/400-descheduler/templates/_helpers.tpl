{{- /*
  Prints "true" when the policy gets the Prometheus metrics provider: the prometheus module is enabled
  and at least one Descheduler runs LowNodeUtilization with metricsSource Prometheus.
  The policy provider, the TLS env of the descheduler and the aggregating-proxy RBAC all follow this condition.
*/}}
{{- define "descheduler_prometheus_metrics_enabled" }}
  {{- $enabled := false }}
  {{- if .Values.global.enabledModules | has "prometheus" }}
    {{- range .Values.descheduler.internal.deschedulers }}
      {{- if and (dig "strategies" "lowNodeUtilization" "enabled" false .) (eq (dig "strategies" "lowNodeUtilization" "metricsSource" "" .) "Prometheus") }}
        {{- $enabled = true }}
      {{- end }}
    {{- end }}
  {{- end }}
  {{- if $enabled }}true{{- end }}
{{- end }}
