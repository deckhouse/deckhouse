#!/bin/bash

if ! uname -a | grep -q hardened; then
  apt update                           && \
  apt install -f linux-latest-hardened && \
  reboot
fi
