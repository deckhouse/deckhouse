#!/bin/bash

# These scripts were modified from boxcutter

set -e
set -x

DISK_USAGE_BEFORE_CLEANUP=$(df -h)

# Make sure udev does not block our network - http://6.ptmc.org/?p=164
# echo "==> Cleaning up udev rules"
# rm -rf /dev/.udev/
# rm /lib/udev/rules.d/75-persistent-net-generator.rules

# Blank machine-id (DUID) so machines get unique ID generated on boot.
# https://www.freedesktop.org/software/systemd/man/machine-id.html#Initialization
echo "==> Blanking systemd machine-id"
if [ -f "/etc/machine-id" ]; then
    truncate -s 0 "/etc/machine-id"
fi

echo "==> Cleaning up tmp"
rm -rf /tmp/*

# Cleanup apt cache
apt-get -y autoremove --purge
apt-get -y clean
apt-get -y autoclean


echo "==> Installed packages"
dpkg --get-selections | grep -v deinstall


# Clean up log files
find /var/log -type f | while read f; do echo -ne '' > "${f}"; done;

echo "==> Clearing last login information"
>/var/log/lastlog
>/var/log/wtmp
>/var/log/btmp

# # Whiteout /boot
# count=$(df --sync -kP /boot | tail -n1 | awk -F ' ' '{print $4}')
# let count--
# dd if=/dev/zero of=/boot/whitespace bs=1024 count=$count
# rm /boot/whitespace

echo '==> Clear out swap and disable until reboot'
set +e
swapuuid=$(/sbin/blkid -o value -l -s UUID -t TYPE=swap)
case "$?" in
    2|0) ;;
    *) exit 1 ;;
esac

set -e
if [ "x${swapuuid}" != "x" ]; then
    # Whiteout the swap partition to reduce box size
    # Swap is disabled till reboot
    swappart=$(readlink -f /dev/disk/by-uuid/$swapuuid)
    /sbin/swapoff "${swappart}"
    dd if=/dev/zero of="${swappart}" bs=1M || echo "dd exit code $? is suppressed"
    /sbin/mkswap -U "${swapuuid}" "${swappart}"
fi

# Zero out the free space to save space in the final image
dd if=/dev/zero of=/EMPTY bs=1M  || echo "dd exit code $? is suppressed"
rm -f /EMPTY

# Make sure we wait until all the data is written to disk, otherwise
# Packer might quite too early before the large files are deleted
sync

echo "==> Disk usage before cleanup"
echo "${DISK_USAGE_BEFORE_CLEANUP}"

echo "==> Disk usage after cleanup"
df -h
