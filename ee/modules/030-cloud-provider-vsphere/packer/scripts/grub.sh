#!/bin/bash

# These scripts were modified from boxcutter

set -e
set -x

if [ $(lsb_release -rs) = "22.04" ] ; then
  rm -f /etc/cloud/cloud.cfg.d/99-installer.cfg
  rm -f /etc/cloud/cloud.cfg.d/subiquity-disable-cloudinit-networking.cfg
  echo 'disable_vmware_customization: false' | tee -a /etc/cloud/cloud.cfg
  sed -i 's|nocloud-net;seedfrom=http://.*/|vmware|' /etc/default/grub
  update-grub
  cloud-init clean
fi
