# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

# There is issue that blkid hangs on nodes with kernel 5.x.x version because of floppy drive presence.
# We don't need floppy drive on kubernetes nodes so we disable it for good.
if [[ -f /etc/modprobe.d/blacklist-floppy.conf ]]; then
  return 0
fi

echo "blacklist floppy" > /etc/modprobe.d/blacklist-floppy.conf
if lsmod | grep floppy -q ; then
    update-initramfs -u
    bb-flag-set reboot
fi
