# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

# hack to avoid problems due to systemd difference for suse and ubuntu
if [[ ! -e /lib/systemd ]]; then
  ln -s /lib/systemd /var/lib/systemd
fi
