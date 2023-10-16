#!/bin/bash
{{- /*
# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/}}
shopt -s extglob

configured_macs="$(grep -Po '(?<=macaddress: ).+' /etc/netplan/50-cloud-init.yaml)"
for mac in $configured_macs; do
  ifname="$(ip -o link show | grep "link/ether $mac" | cut -d ":" -f2 | tr -d " ")|"
  configured_ifnames_pattern+="$ifname"
done
for i in /sys/class/net/!(${configured_ifnames_pattern%?}); do
  if ! udevadm info "$i" 2>/dev/null | grep -Po '(?<=E: ID_NET_DRIVER=)virtio.*' 1>/dev/null 2>&1; then
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
