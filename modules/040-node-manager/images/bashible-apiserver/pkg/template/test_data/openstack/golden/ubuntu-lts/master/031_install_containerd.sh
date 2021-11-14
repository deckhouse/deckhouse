# Copyright 2021 Flant JSC
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

bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  if bb-flag? there-was-containerd-installed; then
    bb-log-info "Setting reboot flag due to containerd package being updated"
    bb-flag-set reboot
    bb-flag-unset there-was-containerd-installed
  fi
  systemctl daemon-reload
  systemctl enable containerd.service
systemctl restart containerd.service
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
  bb-apt-remove docker.io
  bb-rp-remove docker-ce containerd-io
  rm -rf /var/lib/docker/ /var/run/docker.sock /var/lib/containerd/ /etc/docker /etc/containerd/config.toml
  # Pod kubelet-eviction-thresholds-exporter in cri=Docker mode mounts /var/run/containerd/containerd.sock, /var/run/containerd/containerd.sock will be a directory and newly installed containerd won't run. Same thing with crictl.
  rm -rf /var/run/containerd /usr/local/bin/crictl

  bb-log-info "Setting reboot flag due to cri being updated"
  bb-flag-set reboot
fi
if bb-is-ubuntu-version? 16.04 ; then
  desired_version="containerd.io=1.4.6-1"
  allowed_versions_pattern="containerd.io=1.[4]"
fi
if bb-is-ubuntu-version? 18.04 ; then
  desired_version="containerd.io=1.4.6-1"
  allowed_versions_pattern="containerd.io=1.[4]"
fi
if bb-is-ubuntu-version? 20.04 ; then
  desired_version="containerd.io=1.4.6-1"
  allowed_versions_pattern="containerd.io=1.[4]"
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

  if bb-is-ubuntu-version? 16.04 ; then
    containerd_tag="620b47c26662252451f716a6faf9a7f9f4083985614f807c38f16b6e-1638387845865"
  elif bb-is-ubuntu-version? 18.04 ; then
    containerd_tag="f37a6d07434a66a0bb03961127c8ba6c2c72906c22b844aa9d9de1a2-1638387812931"
  elif bb-is-ubuntu-version? 20.04 ; then
    containerd_tag="bad69ba04514d1ed3d99bba060805b86c4cb16b9f59b6f30fa983725-1638387839718"
  else
    bb-log-error "Unsupported ubuntu version"
    exit 1
  fi
  crictl_tag="f8df7211c65ff8ac1eef4b8981c27bf48dc126f1ad556c0a23c3e008-1638042873105"
  containerd_fe_tag="a46aa63ed7043ffdec67861704d1b8e402de1f30bc29b8374d987551-1638870303845"

  bb-rp-install "containerd-io:${containerd_tag}" "crictl:${crictl_tag}" "containerd-flant-edition:${containerd_fe_tag}"

  mkdir -p /etc/systemd/system/containerd.service.d
  bb-sync-file /etc/systemd/system/containerd.service.d/override.conf - << EOF
[Service]
ExecStart=
ExecStart=-/usr/local/bin/containerd
EOF
fi
