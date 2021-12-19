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

{{- $manage_kernel := true }}
{{- if hasKey .nodeGroup "operatingSystem" }}
  {{- if not .nodeGroup.operatingSystem.manageKernel }}
    {{- $manage_kernel = false }}
  {{- end }}
{{- end }}

{{- if $manage_kernel }}
  {{- $desired_version := index .k8s .kubernetesVersion "bashible" "centos" "7" "kernel" "generic" "desiredVersion" }}
  {{- $allowed_versions_pattern := index .k8s .kubernetesVersion "bashible" "centos" "7" "kernel" "generic" "allowedPattern" }}
desired_version={{ $desired_version | quote }}
allowed_versions_pattern={{ $allowed_versions_pattern | quote }}

if [[ -z $desired_version ]]; then
  bb-log-error "Desired version must be set"
  exit 1
fi

should_install_kernel=true

# Do not install kernel if version_in_use is equal to desired version or is allowed.
version_in_use="$(uname -r)"
if test -n "$allowed_versions_pattern" && grep -Eq "$allowed_versions_pattern" <<< "$version_in_use"; then
  should_install_kernel=false
fi
if [[ "$version_in_use" == "$desired_version" ]]; then
  should_install_kernel=false
fi

if [[ "$should_install_kernel" == true ]]; then
  bb-deckhouse-get-disruptive-update-approval
  kernel_tag="{{- index .images.registrypackages (printf "kernelCentos7%s" ($desired_version | replace "." "_" | replace "-" "_" | camelcase )) }}"
  bb-rp-install "kernel:${kernel_tag}"
  {{- if ne .runType "ImageBuilding" }}
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
  {{- end }}
fi

should_run_grub=false

# Workaround for bug https://github.com/docker/for-linux/issues/841 - cannot allocate memory in /sys/fs/cgroup
if ! grep -q "cgroup.memory=nokmem" /etc/default/grub; then
  sed -i "s/GRUB_CMDLINE_LINUX=\"\(.*\)\"/GRUB_CMDLINE_LINUX=\"\1 cgroup.memory=nokmem\"/" /etc/default/grub
  should_run_grub=true
fi

# Set newer kernel as default
if ! grep -q "GRUB_DEFAULT=0" /etc/default/grub; then
  sed -i "s/^GRUB_DEFAULT=.\+$/GRUB_DEFAULT=0/" /etc/default/grub
  should_run_grub=true
fi

if [[ "$should_run_grub" == true ]]; then
  grub2-mkconfig -o /boot/grub2/grub.cfg
  bb-log-info "Setting reboot flag due to grub cmdline for kernel was updated"
  bb-flag-set reboot
fi

{{- end }}
