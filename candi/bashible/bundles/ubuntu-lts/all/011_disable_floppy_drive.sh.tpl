# There is issue that blkid hangs on nodes with kernel 5.x.x version because of floppy drive presence.
# We don't need floppy drive on kubernetes nodes so we disable it for good.
echo "blacklist floppy" > /etc/modprobe.d/blacklist-floppy.conf
if lsmod | grep floppy -q ; then
  if rmmod floppy ; then
    update-initramfs -u
  else
    bb-flag-set reboot
  fi
fi
