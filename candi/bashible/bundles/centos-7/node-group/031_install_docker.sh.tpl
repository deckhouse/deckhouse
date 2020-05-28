bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  if bb-flag? there-was-docker-installed; then
    bb-flag-set reboot
    bb-flag-unset there-was-docker-installed
  fi

  systemctl enable docker.service
{{ if ne .runType "ImageBuilding" -}}
  systemctl restart docker.service
{{- end }}
}

{{- if hasKey .nodeGroup "docker" }}
  {{- if .nodeGroup.docker.nvidia }}
    >&2 echo "ERROR: CentOS nvidia docker is not supported yet!"
    exit 1
  {{- end }}
{{- end }}

docker_package="docker-ce-18.09.9-3.el7.x86_64"
docker_cli_package="docker-ce-cli-18.09.9-3.el7.x86_64"

if bb-yum-package? docker-ce; then
  bb-flag-set there-was-docker-installed
fi

if ! bb-yum-package? $docker_package; then
  bb-deckhouse-get-disruptive-update-approval
fi

bb-yum-install $docker_package $docker_cli_package
