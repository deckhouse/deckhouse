#!/bin/bash

set -e
set -x

if [ $(lsb_release -rs) != "22.04" ] ; then
  curl -sSL https://raw.githubusercontent.com/vmware/cloud-init-vmware-guestinfo/master/install.sh | sh -
fi
