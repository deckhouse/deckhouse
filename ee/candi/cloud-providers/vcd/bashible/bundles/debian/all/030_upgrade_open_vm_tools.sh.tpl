# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

bb-apt-install "open-vm-tools"

bb-event-on 'bb-package-installed' 'restart-open-vm-tools'
restart-open-vm-tools() {
  bb-log-info 'open-vm-tools installed, executing "systemctl restart open-vm-tools.service"'
  systemctl restart open-vm-tools.service
}
