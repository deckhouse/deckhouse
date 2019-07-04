#!/usr/bin/env bash

set -Eeuo pipefail

/config.sh > /etc/coredns/Corefile

dev_name="nodelocaldns"

if ! ip link show "$dev_name" >/dev/null 2>&1
  then
    ip link add "$dev_name" type dummy
    ### Миграция 2019-07-04: https://github.com/deckhouse/deckhouse/merge_requests/862
    ### Эту запись можно будет удалить после переноса всех kubelet'ов обратно на корректный ServiceIP kube-dns
    ip addr add 169.254.20.10/32 dev "$dev_name"
    ip addr add "$KUBE_DNS_SVC_IP"/32 dev "$dev_name"
  else
    if ! ip -json addr show "$dev_name" | jq -re "any(.[].addr_info[]?.local; . == \"$KUBE_DNS_SVC_IP\") and any(.[].addr_info[]?.local; . == \"169.254.20.10\")" >/dev/null 2>&1
      then
        ip addr flush dev "$dev_name"
        ### Миграция 2019-07-04: https://github.com/deckhouse/deckhouse/merge_requests/862
        ### Эту запись можно будет удалить после переноса всех kubelet'ов обратно на корректный ServiceIP kube-dns
        ip addr add 169.254.20.10/32 dev "$dev_name"
        ip addr add "$KUBE_DNS_SVC_IP"/32 dev "$dev_name"
    fi
fi
