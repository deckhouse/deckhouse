#!/bin/bash

if ! uname -a | grep -q hardened; then
  apt update                           && \
  apt install --allow-change-held-packages --allow-downgrades -y linux-latest-hardened linux-hardened && \
  reboot
fi
