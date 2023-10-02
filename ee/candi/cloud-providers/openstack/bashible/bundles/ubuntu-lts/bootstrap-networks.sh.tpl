#!/bin/bash
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
  cat_dev=$1
  cat_metric=$2
  cat_mac=$3
  cat > /etc/netplan/100-cim-"$cat_dev".yaml <<BOOTSTRAP_NETWORK_EOF
network:
  version: 2
  ethernets:
    $cat_dev:
      dhcp4-overrides:
        route-metric: $cat_metric
      match:
        macaddress: $cat_mac
BOOTSTRAP_NETWORK_EOF
}

ip_addr_show_output=$(ip -json addr show)
count_default=$(ip -json route show default | jq length)
if [[ "$count_default" -gt "1" ]]; then
  configured_macs="$(grep -Po '(?<=macaddress: ).+' /etc/netplan/50-cloud-init.yaml)"
  for mac in $configured_macs; do
    ifname="$(echo "$ip_addr_show_output" | jq -re --arg mac "$mac" '.[] | select(.address == $mac) | .ifname')"
    if [[ "$ifname" != "" ]]; then
      configured_ifnames_pattern+="$ifname "
    fi
  done
  count_configured_ifnames=$(echo $configured_ifnames_pattern | wc -w)
  if [[ "$count_configured_ifnames" -gt "1" ]]; then
    set +e
    check_metric=$(grep -Po '(?<=route-metric: ).+' /etc/netplan/50-cloud-init.yaml | wc -l)
    set -e
    if [[ "$check_metric" -eq "0" ]]; then
      metric=100
      for i in $configured_ifnames_pattern; do
        cim_dev=$i
        cim_mac="$(echo "$ip_addr_show_output" | jq -re --arg ifname "$cim_dev" '.[] | select(.ifname == $ifname) | .address')"
        cim_metric=$metric
        metric=$((metric + 100))
        cat_file "$cim_dev" "$cim_metric" "$cim_mac"
      done
      netplan generate
      netplan apply
    fi
  fi
fi

shopt -u extglob
