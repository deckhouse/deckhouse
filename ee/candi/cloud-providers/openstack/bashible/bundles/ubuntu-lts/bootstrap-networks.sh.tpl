#!/bin/bash
set -x
{{- /*
# Copyright 2023 Flant JSC
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

  netmask=$(((0xFFFFFFFF << (32 - net_prefix)) & 0xFFFFFFFF))
  test $((netmask & ip_dec)) -eq $((netmask & net_address_dec))
}

function cat_file() {
  dev=$1
  metric=$2
  mac=$3
  cat > /etc/netplan/100-cim-"$dev".yaml <<BOOTSTRAP_NETWORK_EOF
network:
  version: 2
  ethernets:
    $cim_dev:
      dhcp4-overrides:
        route-metric: $metric
      match:
        macaddress: $mac
BOOTSTRAP_NETWORK_EOF
}

ip_addr_show_output=$(ip -json addr show)
count_default=$(ip -json route show default | jq length)
if [[ "$count_default" != "1" ]]; then
  configured_macs="$(grep -Po '(?<=macaddress: ).+' /etc/netplan/50-cloud-init.yaml)"
  for mac in $configured_macs; do
    ifname="$(echo "$ip_addr_show_output" | jq -re --arg mac "$mac" '.[] | select(.address == $mac) | .ifname')|"
    if [[ "$ifname" != "" ]]; then
      configured_ifnames_pattern+="$ifname"
    fi
  done
  count_configured_ifnames=$(echo $configured_ifnames_pattern | wc -l)
  if [[ "$count_configured_ifnames" != "1" ]]; then
    check_metric=$(grep -Po '(?<=route-metric: ).+' /etc/netplan/50-cloud-init.yaml | wc -l)
    if [[ "$check_metric" != "0" ]]; then
      metric=100
      for i in $configured_ifnames_pattern; do
        cim_dev=$i
        cim_mac="$(echo "$ip_addr_show_output" | jq -re --arg ifname "$ifname" '.[] | select(.ifname == $ifname) | .address')"
        cim_metric=$metric
        metric=$(expr $metric + 100)
        cat_file "$cim_dev" "$cim_metric" "$cim_mac"
        netplan generate
        netplan apply
      done
    fi
  fi
fi

set +x
shopt -u extglob
