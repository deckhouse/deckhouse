{{- if include "node_group_manage_docker" .nodeGroup }}

bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  if bb-flag? there-was-docker-installed; then
    bb-log-info "Setting reboot flag due to docker package was updated"
    bb-flag-set reboot
    bb-flag-unset there-was-docker-installed
  fi

  if bb-is-ubuntu-version? 18.04; then
    systemctl unmask docker.service  # Fix bug in ubuntu 18.04: https://bugs.launchpad.net/ubuntu/+source/docker.io/+bug/1844894
  fi
  systemctl enable docker.service
{{ if ne .runType "ImageBuilding" -}}
  systemctl restart docker.service
{{- end }}
}

# TODO: remove ASAP, provide proper migration from "docker.io" to "docker-ce"
if bb-apt-package? docker.io ; then
  bb-log-warning 'Skipping "docker-ce" installation, since "docker.io" is already installed'
  exit 0
fi

if bb-is-ubuntu-version? 18.04 ; then
  package="docker-ce=5:18.09.7~3-0~ubuntu-bionic"
elif bb-is-ubuntu-version? 16.04 ; then
  package="docker-ce=5:18.09.7~3-0~ubuntu-xenial"
else
  bb-log-error "Unsupported Ubuntu version"
  exit 1
fi

if bb-apt-package? $(echo $package | cut -f1 -d"="); then
  bb-flag-set there-was-docker-installed
fi

if ! bb-apt-package? $package; then
  bb-deckhouse-get-disruptive-update-approval
fi

if bb-apt-package? docker.io; then
  bb-apt-remove docker.io
  bb-flag-set there-was-docker-installed
fi

bb-apt-install $package
{{- end }}
