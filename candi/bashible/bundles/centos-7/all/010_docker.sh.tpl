bb-yum-install 'docker-ce-18.09*' 'docker-ce-cli-18.09*' containerd.io

{{ if ne .runType "ImageBuilding" }}
bb-event-on 'bb-sync-file-changed' '_on_docker_config_changed'
_on_docker_config_changed() {
  systemctl restart docker
}
{{ end }}

mkdir -p /etc/docker
bb-sync-file /etc/docker/daemon.json - << "EOF"
{
        "log-driver": "json-file",
        "log-opts": {
                "max-file": "5",
                "max-size": "10m"
        }
}
EOF
