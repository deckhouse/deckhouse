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

if bb-is-ubuntu-version? 18.04 ; then
  package="docker.io=18.09.7-0ubuntu1~18.04.4"
elif bb-is-ubuntu-version? 16.04 ; then
  package="docker.io=18.09.7-0ubuntu1~16.04.5"
else
  bb-log-error "Unsupported Ubuntu version"
  exit 1
fi

if bb-apt-package? docker.io; then
  bb-flag-set there-was-docker-installed
fi

if ! bb-apt-package? $package; then
  bb-deckhouse-get-disruptive-update-approval
fi

bb-apt-install $package
