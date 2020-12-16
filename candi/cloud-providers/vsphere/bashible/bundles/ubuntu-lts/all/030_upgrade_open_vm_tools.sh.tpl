if bb-is-ubuntu-version? 20.04 ; then
  bb-apt-install "open-vm-tools=2:11.1.5-1~ubuntu20.04.*"
elif bb-is-ubuntu-version? 18.04 ; then
  bb-apt-install "open-vm-tools=2:11.0.5-4ubuntu0.*"
elif bb-is-ubuntu-version? 16.04 ; then
  bb-apt-install "open-vm-tools=2:10.2.0-3~ubuntu0.*"
else
  bb-log-error "Unsupported Ubuntu version"
  exit 1
fi

bb-event-on 'bb-package-installed' 'restart-open-vm-tools'
restart-open-vm-tools() {
  bb-log-info 'open-vm-tools installed, executing "systemctl restart open-vm-tools.service"'
  systemctl restart open-vm-tools.service
}
