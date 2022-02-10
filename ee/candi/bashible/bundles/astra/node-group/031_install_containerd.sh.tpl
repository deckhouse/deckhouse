# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

{{- if eq .cri "Containerd" }}

bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  if bb-flag? there-was-containerd-installed; then
    bb-log-info "Setting reboot flag due to containerd package being updated"
    bb-flag-set reboot
    bb-flag-unset there-was-containerd-installed
  fi
  systemctl daemon-reload
  systemctl enable containerd.service
{{ if ne .runType "ImageBuilding" -}}
  systemctl restart containerd.service
{{- end }}
}

if bb-apt-package? docker-ce || bb-apt-package? docker.io; then
  bb-deckhouse-get-disruptive-update-approval
  systemctl stop kubelet.service
  # Stop docker containers if they run
  docker stop $(docker ps -q) || true
  systemctl stop docker.service
  systemctl stop containerd.service
  # Kill running containerd-shim processes
  kill $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}') 2>/dev/null || true
  # Remove mounts
  umount $(mount | grep "/run/containerd" | cut -f3 -d" ") 2>/dev/null || true
  bb-rp-remove docker-ce containerd-io
  bb-apt-remove docker.io docker-ce containerd-io
  rm -rf /var/lib/docker/ /var/run/docker.sock /var/lib/containerd/ /etc/docker /etc/containerd/config.toml
  # Pod kubelet-eviction-thresholds-exporter in cri=Docker mode mounts /var/run/containerd/containerd.sock, /var/run/containerd/containerd.sock will be a directory and newly installed containerd won't run. Same thing with crictl.
  rm -rf /var/run/containerd /usr/local/bin/crictl

  bb-log-info "Setting reboot flag due to cri being updated"
  bb-flag-set reboot
fi

if bb-is-astra-version? 2.12.+ || bb-is-astra-version? 1.7.+; then
  desired_version={{ index .k8s .kubernetesVersion "bashible" "debian" "9" "containerd" "desiredVersion" | quote }}
  allowed_versions_pattern={{ index .k8s .kubernetesVersion "bashible" "debian" "9" "containerd" "allowedPattern" | quote }}
fi

if [[ -z $desired_version ]]; then
  bb-log-error "Desired version must be set"
  exit 1
fi

should_install_containerd=true
version_in_use="$(dpkg -l containerd.io 2>/dev/null | grep -E "(hi|ii)\s+(containerd.io)" | awk '{print $2"="$3}' || true)"
if test -n "$allowed_versions_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_pattern" <<< "$version_in_use"; then
  should_install_containerd=false
fi

if [[ "$version_in_use" == "$desired_version" ]]; then
  should_install_containerd=false
fi

if [[ "$should_install_containerd" == true ]]; then

  if bb-apt-package? "$(echo $desired_version | cut -f1 -d"=")"; then
    bb-flag-set there-was-containerd-installed
  fi

  bb-deckhouse-get-disruptive-update-approval

  if bb-is-astra-version? 2.12.+ || bb-is-astra-version? 1.7.+ ; then
    containerd_tag="{{- index $.images.registrypackages (printf "containerdDebian%sStretch" (index .k8s .kubernetesVersion "bashible" "debian" "9" "containerd" "desiredVersion" | replace "containerd.io=" "" | replace "." "" | replace "-" "")) }}"
  fi

  crictl_tag="{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}"

  bb-rp-install "containerd-io:${containerd_tag}" "crictl:${crictl_tag}"
fi

# Upgrade containerd-flant-edition if needed
containerd_fe_tag="{{ index .images.registrypackages "containerdFe146" | toString }}"
if ! bb-rp-is-installed? "containerd-flant-edition" "${containerd_fe_tag}" ; then
  systemctl stop containerd.service
  bb-rp-install "containerd-flant-edition:${containerd_fe_tag}"

  mkdir -p /etc/systemd/system/containerd.service.d
  bb-sync-file /etc/systemd/system/containerd.service.d/override.conf - << EOF
[Service]
ExecStart=
ExecStart=-/usr/local/bin/containerd
EOF
fi

{{- end }}
