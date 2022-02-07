# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

{{- if eq .cri "Docker" }}

bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  if bb-flag? there-was-containerd-installed; then
    bb-log-info "Setting reboot flag due to containerd package was updated"
    bb-flag-set reboot
    bb-flag-unset there-was-containerd-installed
  fi

  if bb-flag? there-was-docker-installed; then
    bb-log-info "Setting reboot flag due to docker package was updated"
    bb-flag-set reboot
    bb-flag-unset there-was-docker-installed
  fi

  if bb-flag? new-docker-installed; then
    systemctl enable docker.service
{{ if ne .runType "ImageBuilding" -}}
    systemctl restart docker.service
{{- end }}
    bb-flag-unset new-docker-installed
 fi
}

# TODO: remove ASAP, provide proper migration from "docker.io" to "docker-ce"
if bb-apt-package? docker.io ; then
  bb-log-warning 'Skipping "docker-ce" installation, since "docker.io" is already installed'
  exit 0
fi

if bb-apt-package? containerd.io && ! bb-apt-package? docker-ce ; then
  bb-deckhouse-get-disruptive-update-approval
  systemctl stop kubelet.service
  systemctl stop containerd.service
  # Kill running containerd-shim processes
  kill $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}') 2>/dev/null || true
  # Remove mounts
  umount $(mount | grep "/run/containerd" | cut -f3 -d" ") 2>/dev/null || true
  bb-rp-remove containerd-io crictl containerd-flant-edition
  rm -rf /var/lib/containerd/ /var/run/containerd /etc/containerd/config.toml /etc/systemd/system/containerd.service.d
  # Pod kubelet-eviction-thresholds-exporter in cri=Containerd mode mounts /var/run/docker.sock, /var/run/docker.sock will be a directory and newly installed docker won't run.
  rm -rf /var/run/docker.sock
  systemctl daemon-reload
  bb-log-info "Setting reboot flag due to cri being updated"
  bb-flag-set reboot
fi

if bb-is-astra-version? 2.12.+; then
  desired_version_docker={{ index .k8s .kubernetesVersion "bashible" "debian" "9" "docker" "desiredVersion" | quote }}
  allowed_versions_docker_pattern={{ index .k8s .kubernetesVersion "bashible" "debian" "9" "docker" "allowedPattern" | quote }}
  desired_version_containerd={{ index .k8s .kubernetesVersion "bashible" "debian" "9" "docker" "containerd" "desiredVersion" | quote }}
  allowed_versions_containerd_pattern={{ index .k8s .kubernetesVersion "bashible" "debian" "9" "docker" "containerd" "allowedPattern" | quote }}
fi

if [[ -z $desired_version_docker || -z $desired_version_containerd ]]; then
  bb-log-error "Desired version must be set"
  exit 1
fi

should_install_containerd=true
version_in_use="$(dpkg -l containerd.io 2>/dev/null | grep -E "(hi|ii)\s+(containerd.io)" | awk '{print $2"="$3}' || true)"
if test -n "$allowed_versions_containerd_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_containerd_pattern" <<< "$version_in_use"; then
  should_install_containerd=false
fi

if [[ "$version_in_use" == "$desired_version_containerd" ]]; then
  should_install_containerd=false
fi

if [[ "$should_install_containerd" == true ]]; then

  if bb-apt-package? "$(echo $desired_version_containerd | cut -f1 -d"=")"; then
    bb-flag-set there-was-containerd-installed
  fi

  bb-deckhouse-get-disruptive-update-approval

  if bb-is-astra-version? 2.12.+; then
    containerd_tag="{{- index $.images.registrypackages (printf "containerdDebian%sStretch" (index .k8s .kubernetesVersion "bashible" "debian" "9" "docker" "containerd" "desiredVersion" | replace "containerd.io=" "" | replace "." "" | replace "-" "")) }}"
  fi

  bb-rp-install "containerd-io:${containerd_tag}"
fi

should_install_docker=true
version_in_use="$(dpkg -l docker-ce 2>/dev/null | grep -E "(hi|ii)\s+(docker-ce)" | awk '{print $2"="$3}' || true)"
if test -n "$allowed_versions_docker_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_docker_pattern" <<< "$version_in_use"; then
  should_install_docker=false
fi

if [[ "$version_in_use" == "$desired_version_docker" ]]; then
  should_install_docker=false
fi

if [[ "$should_install_docker" == true ]]; then
  if bb-apt-package? "$(echo $desired_version_docker | cut -f1 -d"=")"; then
    bb-flag-set there-was-docker-installed
  fi

  bb-deckhouse-get-disruptive-update-approval

  bb-flag-set new-docker-installed

  if bb-is-astra-version? 2.12.+; then
    docker_tag="{{- index $.images.registrypackages (printf "dockerDebian%s" (index .k8s .kubernetesVersion "bashible" "debian" "9" "docker" "desiredVersion" | replace "docker-ce=" "" | replace "." "_" | replace ":" "_" | replace "~" "_" | camelcase)) }}"
  fi

  bb-rp-install "docker-ce:${docker_tag}"
fi

{{- end }}
