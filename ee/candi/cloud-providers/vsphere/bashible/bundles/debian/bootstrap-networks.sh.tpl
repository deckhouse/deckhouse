#!/bin/bash
{{- /*
# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/}}
shopt -s extglob

primary_mac="$(grep -Po '(?<=macaddress: ).+' /etc/netplan/50-cloud-init.yaml)"

if [ -z "$primary_mac" ]; then
  primary_ifname=$(grep -Po '(ens|eth|eno|enp)[0-9]+(?=:)' /etc/netplan/50-cloud-init.yaml | head -n1)
else
  primary_ifname="$(ip -o link show | grep "link/ether $primary_mac" | cut -d ":" -f2 | tr -d " ")"
fi

for i in /sys/class/net/!($primary_ifname); do
  if ! udevadm info "$i" 2>/dev/null | grep -Po '(?<=E: ID_NET_DRIVER=)vmxnet.*' 1>/dev/null 2>&1; then
    continue
  fi

  ifname=$(basename "$i")
  mac="$(ip link show dev $ifname | grep "link/ether" | sed "s/  //g" | cut -d " " -f2)"

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
