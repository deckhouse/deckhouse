# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

{{- $manage_kernel := true }}
{{- if hasKey .nodeGroup "operatingSystem" }}
  {{- if not .nodeGroup.operatingSystem.manageKernel }}
    {{- $manage_kernel = false }}
  {{- end }}
{{- end }}

{{- if $manage_kernel }}
  {{- if ne .runType "ImageBuilding" }}
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
}
  {{- end }}

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "alteros" }}
  {{- $alterosVersion := toString $key }}
  {{- if $value.kernel.generic }}
    {{- if or $value.kernel.generic.desiredVersion $value.kernel.generic.allowedPattern }}
if bb-is-alteros-version? {{ $alterosVersion }} ; then
  desired_version={{ $value.kernel.generic.desiredVersion | quote }}
  allowed_versions_pattern={{ $value.kernel.generic.allowedPattern | quote }}
fi
    {{- end }}
  {{- end }}
{{- end }}

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
  bb-yum-install "kernel-${desired_version}"
  packages_to_remove="$(rpm -q kernel | grep -Ev "^kernel-${desired_version}$" || true)"
else
  packages_to_remove="$(rpm -q kernel | grep -Ev "$allowed_versions_pattern" | grep -Ev "^kernel-${version_in_use}$" || true)"
fi

if [ -n "$packages_to_remove" ]; then
  bb-yum-remove $packages_to_remove
fi

# Workaround for bug https://github.com/docker/for-linux/issues/841 - cannot allocate memory in /sys/fs/cgroup
if ! grep -q "cgroup.memory=nokmem" /etc/default/grub; then
  sed -i "s/GRUB_CMDLINE_LINUX=\"\(.*\)\"/GRUB_CMDLINE_LINUX=\"\1 cgroup.memory=nokmem\"/" /etc/default/grub
  grub2-mkconfig -o /boot/grub2/grub.cfg
  bb-log-info "Setting reboot flag due to grub cmdline for kernel was updated"
  bb-flag-set reboot
fi

{{- end }}
