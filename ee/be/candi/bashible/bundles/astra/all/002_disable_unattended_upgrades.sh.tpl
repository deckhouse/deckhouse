# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

if systemctl is-enabled --quiet unattended-upgrades ; then
  systemctl disable --now unattended-upgrades
fi

bb-apt-remove unattended-upgrades
