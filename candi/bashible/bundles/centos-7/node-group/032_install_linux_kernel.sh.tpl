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

if ! bb-yum-package? "kernel-${desired_version}"; then
  bb-deckhouse-get-disruptive-update-approval
  bb-yum-install "kernel-${desired_version}"
fi

# Workaround for bug https://github.com/docker/for-linux/issues/841 - cannot allocate memory in /sys/fs/cgroup
if ! grep -q "cgroup.memory=nokmem" /etc/default/grub; then
  sed -i "s/GRUB_CMDLINE_LINUX=\"\(.*\)\"/GRUB_CMDLINE_LINUX=\"\1 cgroup.memory=nokmem\"/" /etc/default/grub
  grub2-mkconfig -o /boot/grub2/grub.cfg
  bb-log-info "Setting reboot flag due to grub cmdline for kernel was updated"
  bb-flag-set reboot
fi

packages="$(rpm -q kernel | grep -Ev "^kernel-${desired_version}$" || true)"
if [ -n "$packages" ]; then
  bb-yum-remove $packages
fi

  {{- if ne .runType "ImageBuilding" }}
if [[ "$(uname -r)" != "$desired_version" ]]; then
  bb-flag-set reboot
fi
  {{- end }}
{{- end }}
