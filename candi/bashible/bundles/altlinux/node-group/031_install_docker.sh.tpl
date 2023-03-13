# Copyright 2023 Flant JSC
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

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "altlinux" }}
  {{- $altlinuxVersion := toString $key }}
  {{- if or $value.docker.desiredVersion $value.docker.allowedPattern }}
if bb-is-altlinux-version? {{ $altlinuxVersion }} ; then
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
version_in_use="$(containerd --version 2>/dev/null | awk '{print "containerd-"$3}' | sed 's/v//' || true)"
if test -n "$allowed_versions_containerd_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_containerd_pattern" <<< "$version_in_use"; then
  should_install_containerd=false
fi

if [[ "$version_in_use" == "$desired_version_containerd" ]]; then
  should_install_containerd=false
fi

if [[ "$should_install_containerd" == true ]]; then

  if bb-apt-rpm-package? "$(echo $desired_version_containerd | cut -f1 -d"=")"; then
    bb-flag-set there-was-containerd-installed
  fi

  bb-deckhouse-get-disruptive-update-approval

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "altlinux" }}
  {{- $altlinuxVersion := toString $key }}
  if bb-is-altlinux-version? {{ $altlinuxVersion }} ; then
    containerd_tag="{{- index $.images.registrypackages (printf "containerdAltlinux%s" ($value.containerd.desiredVersion | replace "containerd-" "" | replace "." "_" | replace "-" "_" | camelcase )) }}"
  fi
{{- end }}

  bb-rp-install "containerd:${containerd_tag}"
fi

should_install_docker=true
version_in_use="$(rpm -q docker-engine | head -1 || true)"
if test -n "$allowed_versions_docker_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_docker_pattern" <<< "$version_in_use"; then
  should_install_docker=false
fi

if [[ "$version_in_use" == "$desired_version_docker" ]]; then
  should_install_docker=false
fi

if [[ "$should_install_docker" == true ]]; then
  if bb-apt-rpm-package? "$(echo $desired_version_docker | cut -f1 -d"=")"; then
    bb-flag-set there-was-docker-installed
  fi

  bb-deckhouse-get-disruptive-update-approval

  bb-flag-set new-docker-installed

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "altlinux" }}
  {{- $altlinuxVersion := toString $key }}
  if bb-is-altlinux-version? {{ $altlinuxVersion }} ; then
    docker_tag="{{- index $.images.registrypackages (printf "dockerAltlinux%s" ($value.docker.desiredVersion | replace "docker-engine=" "" | replace "." "_" | replace ":" "_" | replace "~" "_" | camelcase)) }}"
  fi
{{- end }}

  bb-rp-install "docker:${docker_tag}"
fi

{{- end }}
