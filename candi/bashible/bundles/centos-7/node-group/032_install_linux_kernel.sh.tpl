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

desired_version="3.10.0-1127.8.2.el7.x86_64"
allowed_versions_pattern=""

should_install_kernel=true
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
  packages_to_remove="$(rpm -q kernel | grep -Ev "^kernel-${version_in_use}$" || true)"
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
