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
      dhcp4: true
      dhcp4-overrides:
        use-hostname: false
        route-metric: $metric
        use-dns: false
        use-ntp: false
      match:
        macaddress: $mac
BOOTSTRAP_NETWORK_EOF
}

ip_route_show_default_output=$(ip -json route show default)
count_default=$(echo $ip_route_show_default_output | jq length)
if [[ "$count_default" != "1" ]]; then
  cim_dev=""
  apiserverEndpoint={{ index (.normal.apiserverEndpoints | first | split ":") "_0" }}
  ip_addr=$(ip -json addr)
  for i in  { 1 .. $count_default }; do
    i_dev=$(echo $ip_route_show_default_output | jq -r .[$i-1].dev)
    i_addr_info=$(echo $ip_addr | jq '.[] | select(.ifname == "'$i_dev'") | .addr_info')
    i_count_addr_info=$(echo $i_addr_info | jq length)
    for j in { 1 .. $i_count_addr_info}; do
      j_local=$(echo $i_addr_info | jq -r .[$j-1].local)
      j_prefixlen=$(echo $i_addr_info | jq -r .[$j-1].prefixlen)
      if is_ip_in_cidr "$apiserverEndpoint" "$j_local/$j_prefixlen"; then
        cim_dev=$i_dev
      fi
    done
  done
  if [[ "$cim_dev" != "" ]]; then
    metric=$(echo $ip_route_show_default_output | jq '.[] | select(.dev == "'$cim_dev'") | .metric')
    cim_metric=""
    cim_mac=""
    for i in  { 1 .. $count_default }; do
      i_dev=$(echo $ip_route_show_default_output | jq -r .[$i-1].dev)
      if [[ "$i_dev" != "$cim_dev"]]; then
        i_metric=$(echo $ip_route_show_default_output | jq '.[] | select(.dev == "'$i_dev'") | .metric')
        if [[ "$i_metric" == "$metric" ]]; then
          cim_mac=$(echo $ip_addr | jq -r '.[] | select(.ifname == "'$cim_dev'") | .address')
          cim_metric=$(expr $metric + 100)
        fi
      fi
    done
    if [[ "$cim_metric" != "" ]]; then
      cat_file "$cim_dev" "$cim_metric" "$cim_mac"
      netplan generate
      netplan apply
    fi
  fi
fi

shopt -u extglob
