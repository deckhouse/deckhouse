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

if bb-is-ubuntu-version? 18.04 ; then
  package="docker.io=18.09.7-0ubuntu1~18.04.4"
elif bb-is-ubuntu-version? 16.04 ; then
  package="docker.io=18.09.7-0ubuntu1~16.04.5"
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

if bb-apt-package? docker-ce; then
  bb-apt-remove docker-ce
  bb-flag-set there-was-docker-installed
fi

if bb-apt-package? docker-ce-cli; then
  bb-apt-remove docker-ce-cli
fi

if bb-apt-package? containerd.io; then
  bb-apt-remove containerd.io
fi

bb-apt-install $package
{{- end }}
