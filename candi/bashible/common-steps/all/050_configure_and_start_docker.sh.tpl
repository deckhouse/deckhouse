bb-event-on 'bb-sync-file-changed' '_on_docker_config_changed'
_on_docker_config_changed() {
{{ if ne .runType "ImageBuilding" -}}
  systemctl restart docker.service
{{- end }}
}

bb-sync-file /etc/docker/daemon.json - << "EOF"
{
        "log-driver": "json-file",
        "log-opts": {
                "max-file": "5",
                "max-size": "10m"
        }
}
EOF
