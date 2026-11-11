{{- /* The NTP servers of the boot-time clock step, as a JSON list: host names and IPv4 addresses
alone. On bashible nodes they go into a unit file written from a heredoc. */}}
{{- define "early_time_sync_servers" }}
  {{- $servers := list }}
  {{- range .Values.chrony.ntpServers }}
    {{- if regexMatch "^[0-9A-Za-z._-]+$" . }}
      {{- $servers = append $servers . }}
    {{- end }}
  {{- end }}
  {{- $servers | toJson }}
{{- end }}
