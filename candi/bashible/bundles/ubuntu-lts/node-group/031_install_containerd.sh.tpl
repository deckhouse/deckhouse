{{- if eq .cri "Containerd" }}

bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  if bb-flag? there-was-containerd-installed; then
    bb-log-info "Setting reboot flag due to containerd package being updated"
    bb-flag-set reboot
    bb-flag-unset there-was-containerd-installed
  fi
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
  bb-apt-remove docker.io docker-ce docker-ce-cli containerd.io
  rm -rf /var/lib/docker/ /var/run/docker.sock /var/lib/containerd/ /etc/docker
  # Pod kubelet-eviction-thresholds-exporter in cri=Docker mode mounts /var/run/containerd/containerd.sock, /var/run/containerd/containerd.sock will be a directory and newly installed containerd won't run. Same thing with crictl.
  rm -rf /var/run/containerd /usr/local/bin/crictl

  bb-log-info "Setting reboot flag due to cri being updated"
  bb-flag-set reboot
fi

desired_version="containerd.io=1.4.3-1"
allowed_versions_pattern=""

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
  bb-apt-install $desired_version

  VERSION="v1.20.0"
  curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-${VERSION}-linux-amd64.tar.gz --output crictl-${VERSION}-linux-amd64.tar.gz
  tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
  rm -f crictl-$VERSION-linux-amd64.tar.gz
fi

{{- end }}
