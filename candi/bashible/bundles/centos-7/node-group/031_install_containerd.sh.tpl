# Copyright 2021 Flant CJSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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

if bb-yum-package? docker-ce; then
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
  bb-yum-remove docker-ce docker-ce-cli containerd.io
  rm -rf /var/lib/docker/ /var/run/docker.sock /var/lib/containerd/ /etc/docker /etc/containerd/config.toml
  # Pod kubelet-eviction-thresholds-exporter in cri=Docker mode mounts /var/run/containerd/containerd.sock, /var/run/containerd/containerd.sock will be a directory and newly installed containerd won't run. Same thing with crictl.
  rm -rf /var/run/containerd /usr/local/bin/crictl

  bb-log-info "Setting reboot flag due to cri being updated"
  bb-flag-set reboot
fi

desired_version={{ index .k8s .kubernetesVersion "bashible" "centos" "7" "containerd" "desiredVersion" | quote }}
allowed_versions_pattern={{ index .k8s .kubernetesVersion "bashible" "centos" "7" "containerd" "allowedPattern" | quote }}

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
  container_selinux_package="container-selinux-2.119.2-1.911c772.el7_8"

  if bb-yum-package? containerd.io; then
    bb-flag-set there-was-containerd-installed
  fi

  bb-deckhouse-get-disruptive-update-approval

  # RHEL 7 hack — containerd.io package requires container-selinux >= 2.9 but it doesn't exist in rhel repos.
  . /etc/os-release
  if [[ "${ID}" == "rhel" ]] && ! bb-yum-package? "$container_selinux_package"; then
    yum install -y "http://mirror.centos.org/centos/7/extras/x86_64/Packages/$container_selinux_package.noarch.rpm"
  fi

  bb-yum-install $container_selinux_package device-mapper-persistent-data lvm $desired_version

  VERSION="v{{ .kubernetesVersion }}.0"
  curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-${VERSION}-linux-amd64.tar.gz --output crictl-${VERSION}-linux-amd64.tar.gz
  tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
  rm -f crictl-$VERSION-linux-amd64.tar.gz

  VERSION_WERF_CONTAINERD="v1.4.6+werf-fix.2"
  CHECKSUM_WERF_CONTAINERD="acafa9285a7b10b6e64c15fb271df1aba17c18898c4d85371d65c30ca7bb17ce"
  curl -L https://github.com/flant/containerd/releases/download/$VERSION_WERF_CONTAINERD/containerd --output /usr/local/bin/containerd
  echo "$CHECKSUM_WERF_CONTAINERD /usr/local/bin/containerd" | sha256sum -c
  chmod +x /usr/local/bin/containerd
  mkdir -p /etc/systemd/system/containerd.service.d
  bb-sync-file /etc/systemd/system/containerd.service.d/override.conf - << EOF
[Service]
ExecStart=
ExecStart=-/usr/local/bin/containerd
EOF
fi

{{- end }}
