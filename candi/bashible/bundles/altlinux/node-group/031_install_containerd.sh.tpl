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

{{- if eq .cri "Containerd" }}

bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  systemctl daemon-reload
  systemctl enable containerd.service

{{ if ne .runType "ImageBuilding" -}}
  bb-flag-set containerd-need-restart
{{- end }}
}


{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "altlinux" }}
  {{- $altlinuxVersion := toString $key }}
  {{- if or $value.containerd.desiredVersion $value.containerd.allowedPattern }}
if bb-is-altlinux-version? {{ $altlinuxVersion }} ; then
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
version_in_use="$(containerd --version 2>/dev/null | awk '{print "containerd-"$3}' | sed 's/v//' || true)"
if test -n "$allowed_versions_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_pattern" <<< "$version_in_use"; then
  should_install_containerd=false
fi

if [[ "$version_in_use" == "$desired_version" ]]; then
  should_install_containerd=false
fi

if [[ "$should_install_containerd" == true ]]; then

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "altlinux" }}
  {{- $altlinuxVersion := toString $key }}
  if bb-is-altlinux-version? {{ $altlinuxVersion }} ; then
    containerd_tag="{{- index $.images.registrypackages (printf "containerdAltlinux%s" ($value.containerd.desiredVersion | replace "containerd-" "" | replace "." "_" | replace "-" "_" | camelcase )) }}"
  fi
{{- end }}

  bb-rp-install "containerd:${containerd_tag}"
fi

# install crictl
crictl_tag="{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}"
if ! bb-rp-is-installed? "crictl" "${crictl_tag}" ; then
  bb-rp-install "crictl:${crictl_tag}"
fi
{{- end }}
