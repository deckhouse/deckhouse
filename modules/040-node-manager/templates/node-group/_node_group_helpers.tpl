{{- define "node_group_manage_docker" -}}
  {{- $ng := . -}}

  {{- $manage_docker := true -}}
  {{- if hasKey $ng "docker" -}}
    {{- if hasKey $ng.docker "manage" -}}
      {{- if not $ng.docker.manage -}}
        {{ $manage_docker = false }}
      {{- end -}}
    {{- end -}}
  {{- end -}}

  {{- if $manage_docker -}}
    not empty string
  {{- end -}}
{{- end -}}
