#!/bin/bash

set -Eeo pipefail

if [[ $1 == "--config" ]] ; then
  cat << EOF
{
  "onStartup": 1
}
EOF
else
  while ! nc -z 169.254.20.10 53 -w 1; do

    # Setup interface
    dev_name="nodelocaldns"
    if ! ip link show "$dev_name" >/dev/null 2>&1 ; then
      ip link add "$dev_name" type dummy
    fi
    if ! ip -json addr show "$dev_name" | jq -re "any(.[].addr_info[]?.local; . == \"169.254.20.10\")" >/dev/null 2>&1 ; then
      ip addr add 169.254.20.10/32 dev "$dev_name"
    fi
    if ! ip -json addr show "$dev_name" | jq -re "any(.[].addr_info[]?.local; . == \"${KUBE_DNS_SVC_IP}\")" >/dev/null 2>&1 ; then
      ip addr add "${KUBE_DNS_SVC_IP}"/32 dev "$dev_name"
    fi

    # Setup iptables
    if ! iptables -w 600 -t raw -C OUTPUT -s ${KUBE_DNS_SVC_IP}/32 -p tcp -m tcp --sport 53 -j NOTRACK >/dev/null 2>&1; then
      iptables -w 600 -t raw -A OUTPUT -s ${KUBE_DNS_SVC_IP}/32 -p tcp -m tcp --sport 53 -j NOTRACK
    fi
    if ! iptables -w 600 -t raw -C OUTPUT -s ${KUBE_DNS_SVC_IP}/32 -p udp -m udp --sport 53 -j NOTRACK >/dev/null 2>&1; then
      iptables -w 600 -t raw -A OUTPUT -s ${KUBE_DNS_SVC_IP}/32 -p udp -m udp --sport 53 -j NOTRACK
    fi
    if iptables -w 600 -t raw -C PREROUTING -d ${KUBE_DNS_SVC_IP}/32 -m socket --nowildcard -j NOTRACK >/dev/null 2>&1 ; then
      # Remove. Will be added later, in nanny.sh
      iptables -w 600 -t raw -D PREROUTING -d ${KUBE_DNS_SVC_IP}/32 -m socket --nowildcard -j NOTRACK >/dev/null 2>&1
    fi

    ### Миграция 2019-10-08: https://github.com/deckhouse/deckhouse/merge_requests/1259
    # Удалить данный кусок скрипта можно после выката MR'а
    if iptables -w 600 -t raw -C OUTPUT -s ${KUBE_DNS_SVC_IP}/32 -j NOTRACK >/dev/null 2>&1; then
      iptables -w 600 -t raw -D OUTPUT -s ${KUBE_DNS_SVC_IP}/32 -j NOTRACK
    fi

    ### Миграция 2019-09-14: https://github.com/deckhouse/deckhouse/merge_requests/1131/diffs
    # Удалить данный кусок скрипта можно после выката MR'а
    if iptables -w 600 -t nat -C PREROUTING -d "$KUBE_DNS_SVC_IP/32" -m socket --nowildcard -j ACCEPT >/dev/null 2>&1 ; then
      iptables -w 600 -t nat -D PREROUTING -d "$KUBE_DNS_SVC_IP/32" -m socket --nowildcard -j ACCEPT >/dev/null 2>&1
    fi
    break
  done
fi
