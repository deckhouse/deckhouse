# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

{{- if eq .cri "Containerd" }}

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
  rm -f /var/lib/cni/networks/cbr0/*
fi

bb-rp-install "containerd:{{- index $.images.registrypackages "containerd1620" }}" "crictl:{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}" "toml-merge:{{ .images.registrypackages.tomlMerge01 }}"
{{- end }}
