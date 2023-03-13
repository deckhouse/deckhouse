#!/bin/bash
{{- /*
# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/}}
shopt -s extglob

ip_addr_show_output=$(ip -json addr show)
primary_mac="$(grep -Po '(?<=macaddress: ).+' /etc/netplan/50-cloud-init.yaml)"
primary_ifname="$(echo "$ip_addr_show_output" | jq -re --arg mac "$primary_mac" '.[] | select(.address == $mac) | .ifname')"

if [ -z "$primary_ifname" ]; then
  primary_ifname="$(cat /etc/netplan/50-cloud-init.yaml  | grep "^ *ens[0-9]*:$" | awk '{sub(/:/,"");print $1}')"
fi

for i in /sys/class/net/!($primary_ifname); do
  if ! udevadm info "$i" 2>/dev/null | grep -Po '(?<=E: ID_NET_DRIVER=)vmxnet.*' 1>/dev/null 2>&1; then
    continue
  fi

  ifname=$(basename "$i")
  mac="$(echo "$ip_addr_show_output" | jq -re --arg ifname "$ifname" '.[] | select(.ifname == $ifname) | .address')"

  cat > /etc/netplan/100-cim-"$ifname".yaml <<BOOTSTRAP_NETWORK_EOF
network:
  version: 2
  ethernets:
    $ifname:
      dhcp4: true
      dhcp4-overrides:
        use-hostname: false
        use-routes: false
        use-dns: false
        use-ntp: false
      match:
        macaddress: $mac
BOOTSTRAP_NETWORK_EOF
done

netplan generate
netplan apply

shopt -u extglob
