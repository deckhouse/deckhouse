{{- define "in_cluster_proxy_enable" -}}
{{- $ctx := . -}}
{{- with $ctx.Values.systemRegistry.internal.orchestrator -}}
    {{- with ((.state).in_cluster_proxy).config -}}
        "not empty string"
    {{- end -}}
{{- end -}}
{{- end -}}
