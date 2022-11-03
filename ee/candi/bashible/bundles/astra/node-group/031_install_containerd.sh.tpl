# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

{{- if eq .cri "Containerd" }}

bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  systemctl daemon-reload
  systemctl enable containerd.service
{{ if ne .runType "ImageBuilding" -}}
  systemctl restart containerd.service
{{- end }}
}

if bb-apt-package? docker-ce || bb-apt-package? docker.io; then
  bb-deckhouse-get-disruptive-update-approval
  if systemctl is-active -q kubelet.service; then
    systemctl stop kubelet.service
  fi

  bb-log-info "Setting reboot flag due to cri being updated"
  bb-flag-set reboot

  # Stop docker containers if they run
  docker stop $(docker ps -q) || true
  systemctl stop docker.service
  systemctl stop containerd.service
  # Kill running containerd-shim processes
  kill $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}') 2>/dev/null || true
  kill -9 $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}') 2>/dev/null || true
  # Remove mounts
  umount $(mount | grep "/run/containerd" | cut -f3 -d" ") 2>/dev/null || true
  bb-rp-remove docker-ce containerd-io
  bb-apt-remove docker.io docker-ce containerd-io
  rm -rf /var/lib/containerd/ /etc/docker /etc/containerd/config.toml
  # Old version of pod kubelet-eviction-thresholds-exporter in cri=Docker mode mounts /var/run/containerd/containerd.sock, /var/run/containerd/containerd.sock will be a directory and newly installed containerd won't run. Same thing with crictl.
  rm -rf /var/run/containerd /usr/local/bin/crictl
  rm -rf /var/lib/docker/ /var/run/docker.sock
fi

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "astra" }}
  {{- $astraVersion := toString $key }}
  {{- if or $value.containerd.desiredVersion $value.containerd.allowedPattern }}
if bb-is-astra-version? {{ $astraVersion }} ; then
  desired_version={{ $value.containerd.desiredVersion | quote }}
  allowed_versions_pattern={{ $value.containerd.allowedPattern | quote }}
fi
  {{- end }}
{{- end }}

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
{{- $astraName := dict "1.7" "Buster" }}
{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "astra" }}
  {{- $astraVersion := toString $key }}
  if bb-is-astra-version? {{ $astraVersion }} ; then
    containerd_tag="{{- index $.images.registrypackages (printf "containerdAstra%s%s" ($value.containerd.desiredVersion | replace "containerd.io=" "" | replace "." "" | replace "-" "") (index $astraName $astraVersion)) }}"
  fi
{{- end }}

  bb-rp-install "containerd-io:${containerd_tag}"
fi

# install crictl
crictl_tag="{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}"
if ! bb-rp-is-installed? "crictl" "${crictl_tag}" ; then
  bb-rp-install "crictl:${crictl_tag}"
fi

# Upgrade containerd-flant-edition if needed
containerd_fe_tag="{{ index .images.registrypackages "containerdFe1511" | toString }}"
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
