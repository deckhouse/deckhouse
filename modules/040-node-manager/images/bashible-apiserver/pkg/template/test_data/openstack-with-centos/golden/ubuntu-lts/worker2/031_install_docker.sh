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
    if bb-is-ubuntu-version? 18.04; then
      systemctl unmask docker.service  # Fix bug in ubuntu 18.04: https://bugs.launchpad.net/ubuntu/+source/docker.io/+bug/1844894
    fi
    systemctl enable docker.service
systemctl restart docker.service
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
if bb-is-ubuntu-version? 16.04 ; then
  desired_version_docker="docker-ce=5:18.09.7~3-0~ubuntu-xenial"
  allowed_versions_docker_pattern=""
  desired_version_containerd="containerd.io=1.4.6-1"
  allowed_versions_containerd_pattern="containerd.io=1.[234]"
fi
if bb-is-ubuntu-version? 18.04 ; then
  desired_version_docker="docker-ce=5:19.03.13~3-0~ubuntu-bionic"
  allowed_versions_docker_pattern="docker-ce=5:18.09.7~3-0~ubuntu-bionic"
  desired_version_containerd="containerd.io=1.4.6-1"
  allowed_versions_containerd_pattern="containerd.io=1.[234]"
fi
if bb-is-ubuntu-version? 20.04 ; then
  desired_version_docker="docker-ce=5:19.03.13~3-0~ubuntu-focal"
  allowed_versions_docker_pattern=""
  desired_version_containerd="containerd.io=1.4.6-1"
  allowed_versions_containerd_pattern="containerd.io=1.[234]"
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

  docker_tag="95f73d35a292ba0739eb43f8522641a783ee944a809f121ae9eb9d92-1638882554095"

  bb-rp-install "docker-ce:${docker_tag}"
fi
