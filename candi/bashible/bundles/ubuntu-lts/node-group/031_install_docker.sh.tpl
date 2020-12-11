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

if bb-is-ubuntu-version? 20.04 ; then
  desired_version="docker-ce=5:19.03.13~3-0~ubuntu-focal"
  allowed_versions_pattern=""
elif bb-is-ubuntu-version? 18.04 ; then
{{- if eq .kubernetesVersion "1.19" }}
  desired_version="docker-ce=5:19.03.13~3-0~ubuntu-bionic"
  allowed_versions_pattern="docker-ce=5:18.09.7~3-0~ubuntu-bionic"
{{- else }}
  desired_version="docker-ce=5:18.09.7~3-0~ubuntu-bionic"
  allowed_versions_pattern=""
{{- end }}
elif bb-is-ubuntu-version? 16.04 ; then
  desired_version="docker-ce=5:18.09.7~3-0~ubuntu-xenial"
  allowed_versions_pattern=""
else
  bb-log-error "Unsupported Ubuntu version"
  exit 1
fi

should_install_docker=true
version_in_use="$(dpkg -l docker-ce 2>/dev/null | grep -E "(hi|ii)\s+(docker-ce)" | awk '{print $2"="$3}' || true)"
if test -n "$allowed_versions_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_pattern" <<< "$version_in_use"; then
  should_install_docker=false
fi

if [[ "$version_in_use" == "$desired_version" ]]; then
  should_install_docker=false
fi

if [[ "$should_install_docker" == true ]]; then
  desired_version_cli="$(sed 's/docker-ce/docker-ce-cli/' <<< "$desired_version")"

  if bb-apt-package? "$(echo $desired_version | cut -f1 -d"=")"; then
    bb-flag-set there-was-docker-installed
  fi

  bb-deckhouse-get-disruptive-update-approval

  if bb-apt-package? docker.io; then
    bb-apt-remove docker.io
    bb-flag-set there-was-docker-installed
  fi

  bb-apt-install $desired_version $desired_version_cli
fi

{{- end }}
