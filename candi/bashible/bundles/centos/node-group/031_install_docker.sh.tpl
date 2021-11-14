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

if bb-yum-package? containerd.io && ! bb-yum-package? docker-ce ; then
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

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "centos" }}
  {{- $centosVersion := toString $key }}
  {{- if or $value.docker.desiredVersion $value.docker.allowedPattern }}
if bb-is-centos-version? {{ $centosVersion }} ; then
  desired_version_docker={{ $value.docker.desiredVersion | quote }}
  allowed_versions_docker_pattern={{ $value.docker.allowedPattern | quote }}
    {{- if or $value.docker.containerd.desiredVersion $value.docker.containerd.allowedPattern }}
  desired_version_containerd={{ $value.docker.containerd.desiredVersion | quote }}
  allowed_versions_containerd_pattern={{ $value.docker.containerd.allowedPattern | quote }}
    {{- end }}
fi
  {{- end }}
{{- end }}

if [[ -z $desired_version_docker || -z $desired_version_containerd ]]; then
  bb-log-error "Desired version must be set"
  exit 1
fi

should_install_containerd=true
version_in_use="$(rpm -q containerd.io | head -1 || true)"
if test -n "$allowed_versions_containerd_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_containerd_pattern" <<< "$version_in_use"; then
  should_install_containerd=false
fi

if [[ "$version_in_use" == "$desired_version_containerd" ]]; then
  should_install_containerd=false
fi

if [[ "$should_install_containerd" == true ]]; then

  if bb-yum-package? containerd.io; then
    bb-flag-set there-was-containerd-installed
  fi

  bb-deckhouse-get-disruptive-update-approval

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "centos" }}
  {{- $centosVersion := toString $key }}
  if bb-is-centos-version? {{ $centosVersion }} ; then
    containerd_tag="{{- index $.images.registrypackages (printf "containerdCentos%s" ($value.docker.containerd.desiredVersion | replace "containerd.io-" "" | replace "." "_" | replace "-" "_" | camelcase )) }}"
  fi
{{- end }}

  bb-rp-install "containerd-io:${containerd_tag}"
fi

should_install_docker=true
version_in_use="$(rpm -q docker-ce | head -1 || true)"
if test -n "$allowed_versions_docker_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_docker_pattern" <<< "$version_in_use"; then
  should_install_docker=false
fi

if [[ "$version_in_use" == "$desired_version_docker" ]]; then
  should_install_docker=false
fi

if [[ "$should_install_docker" == true ]]; then
  if bb-yum-package? docker-ce; then
    bb-flag-set there-was-docker-installed
  fi

  bb-deckhouse-get-disruptive-update-approval

  bb-flag-set new-docker-installed

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "centos" }}
  {{- $centosVersion := toString $key }}
  if bb-is-centos-version? {{ $centosVersion }} ; then
    docker_tag="{{- index $.images.registrypackages (printf "dockerCentos%s%s" ($value.docker.desiredVersion | replace "docker-ce-" "" | replace "." "_" | replace ":" "_" | camelcase ) $key) }}"
  fi
{{- end }}

  bb-rp-install "docker-ce:${docker_tag}"
fi

{{- end }}
