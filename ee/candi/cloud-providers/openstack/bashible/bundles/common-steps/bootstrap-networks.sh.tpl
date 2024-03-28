#!/bin/bash
{{- /*
# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/}}
shopt -s extglob

if ! which netplan 2>/dev/null 1>&2; then
  exit 0
fi

function render_and_deploy_netplan_config() {
  interface=$1
  metric=$2
  mac=$3
  cat > /etc/netplan/100-cim-"$interface".yaml <<EOF
network:
  version: 2
  ethernets:
    $interface:
      dhcp4-overrides:
        route-metric: $metric
      match:
        macaddress: $mac
EOF
}

count_default_routes=$(ip -4 route show default | wc -l)
if [[ "$count_default_routes" -gt "1" ]]; then
  CLOUD_INIT_NETPLAN_CFG="/etc/netplan/50-cloud-init.yaml"
  configured_macs="$(grep -Po '(?<=macaddress: ).+' $CLOUD_INIT_NETPLAN_CFG)"
  for configured_mac in $configured_macs; do
    ifname="$(ip -o link show | grep "link/ether $configured_mac" | cut -d ":" -f2 | tr -d " ")"
    if [[ "$ifname" != "" ]]; then
      configured_ifnames_pattern+="$ifname "
    fi
  done
  count_configured_ifnames=$(wc -w <<< "$configured_ifnames_pattern")
  if [[ "$count_configured_ifnames" -gt "1" ]]; then
    set +e
    check_metric=$(grep -Po '(?<=route-metric: ).+' $CLOUD_INIT_NETPLAN_CFG | wc -l)
    set -e
    if [[ "$check_metric" -eq "0" ]]; then
      global_metric=100
      for i in $configured_ifnames_pattern; do
        cim_dev=$i
        cim_mac="$(ip link show dev $ifname | grep "link/ether" | sed "s/  //g" | cut -d " " -f2)"
        cim_metric=$global_metric
        global_metric=$((global_metric + 100))
        render_and_deploy_netplan_config "$cim_dev" "$cim_metric" "$cim_mac"
      done
      netplan generate
      netplan apply
    fi
  fi
fi

shopt -u extglob
