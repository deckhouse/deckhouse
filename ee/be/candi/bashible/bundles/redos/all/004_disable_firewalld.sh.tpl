# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

if systemctl is-active -q firewalld; then
  systemctl stop firewalld
fi

if systemctl is-enabled -q firewalld; then
  systemctl disable firewalld
  systemctl mask firewalld
fi
