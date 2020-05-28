bb-event-on 'bb-sync-file-changed' '_on_docker_config_changed'
_on_docker_config_changed() {
{{ if ne .runType "ImageBuilding" -}}
  bb-deckhouse-get-disruptive-update-approval
  systemctl restart docker.service
{{- end }}
}

{{- $nvidia_docker := false }}
{{- if hasKey .nodeGroup "docker" }}
  {{- if .nodeGroup.docker.nvidia }}
    {{- $nvidia_docker = true }}
  {{- end }}
{{- end }}

bb-sync-file /etc/docker/daemon.json - << "EOF"
{
{{- $max_concurrent_downloads := 3 }}
{{- if hasKey .nodeGroup "docker" }}
  {{- $max_concurrent_downloads = .nodeGroup.docker.maxConcurrentDownloads | default $max_concurrent_downloads }}
{{- end }}
        "log-driver": "json-file",
        "log-opts": {
                "max-file": "5",
                "max-size": "10m"
        },
{{- if $nvidia_docker }}
        "runtimes": {
          "nvidia": {
            "path": "/usr/bin/nvidia-container-runtime",
            "runtimeArgs": []
          }
        },
        "default-runtime": "nvidia",
{{- end }}
	"max-concurrent-downloads": {{ $max_concurrent_downloads }}
}
EOF
