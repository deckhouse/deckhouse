#!/bin/bash

if [[ -e "/etc/netplan/51-nameservers.yaml" ]]; then
  rm -f "/etc/netplan/51-nameservers.yaml"
  netplan apply
fi
