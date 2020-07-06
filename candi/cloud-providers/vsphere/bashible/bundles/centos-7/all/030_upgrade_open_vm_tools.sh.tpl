bb-yum-install "open-vm-tools-0:10.3.*"

bb-event-on 'bb-package-installed' 'restart-open-vm-tools'
restart-open-vm-tools() {
  bb-log-info 'open-vm-tools installed, executing "systemctl restart open-vm-tools.service"'
  systemctl restart open-vm-tools.service
}
