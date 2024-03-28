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

count_default=$(ip route show default | wc -l)
if [[ "$count_default" -gt "1" ]]; then
  configured_macs="$(grep -Po '(?<=macaddress: ).+' /etc/netplan/50-cloud-init.yaml)"
  for mac in $configured_macs; do
    ifname="$(ip -o link show | grep "link/ether $mac" | cut -d ":" -f2 | tr -d " ")"
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
        cim_mac="$(ip link show dev $ifname | grep "link/ether" | sed "s/  //g" | cut -d " " -f2)"
        cim_metric=$metric
        metric=$((metric + 100))
        render_and_deploy_netplan_config "$cim_dev" "$cim_metric" "$cim_mac"
      done
      netplan generate
      netplan apply
    fi
  fi
fi

shopt -u extglob
