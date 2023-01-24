# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

{{- if eq .cri "Containerd" }}

# install toml-merge
bb-rp-install "toml-merge:{{ .images.registrypackages.tomlMerge01 }}"

bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  systemctl daemon-reload
  systemctl enable containerd.service
{{ if ne .runType "ImageBuilding" -}}
  bb-flag-set containerd-need-restart
{{- end }}
}

if bb-yum-package? docker-ce; then
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
  bb-yum-remove docker.io docker-ce containerd-io
  rm -rf /var/lib/containerd/ /etc/docker /etc/containerd/config.toml
  # Old version of pod kubelet-eviction-thresholds-exporter in cri=Docker mode mounts /var/run/containerd/containerd.sock, /var/run/containerd/containerd.sock will be a directory and newly installed containerd won't run. Same thing with crictl.
  rm -rf /var/run/containerd /usr/local/bin/crictl
  rm -rf /var/lib/docker/ /var/run/docker.sock
fi

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "redos" }}
  {{- $redosVersion := toString $key }}
  {{- if or $value.containerd.desiredVersion $value.containerd.allowedPattern }}
if bb-is-redos-version? {{ $redosVersion }} ; then
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
version_in_use="$(rpm -q containerd.io | head -1 || true)"
if test -n "$allowed_versions_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_pattern" <<< "$version_in_use"; then
  should_install_containerd=false
fi

if [[ "$version_in_use" == "$desired_version" ]]; then
  should_install_containerd=false
fi

if [[ "$should_install_containerd" == true ]]; then

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "redos" }}
  {{- $redosVersion := toString $key }}
  if bb-is-redos-version? {{ $redosVersion }} ; then
    containerd_tag="{{- index $.images.registrypackages (printf "containerdRedos%s" ($value.containerd.desiredVersion | replace "containerd.io-" "" | replace "." "_" | replace "-" "_" | camelcase )) }}"
  fi
{{- end }}

  bb-rp-install "containerd-io:${containerd_tag}"
fi

# install crictl
bb-rp-install "crictl:{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}"
{{- end }}
