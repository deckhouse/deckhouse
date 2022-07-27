#!/bin/bash

# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

set -Eeo pipefail

function setup_iptables() {
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
  if ! iptables -w 60 -t raw -C OUTPUT -s "${KUBE_DNS_SVC_IP}/32" -p tcp -m tcp --sport 53 -j NOTRACK >/dev/null 2>&1; then
    iptables -w 60 -t raw -A OUTPUT -s "${KUBE_DNS_SVC_IP}/32" -p tcp -m tcp --sport 53 -j NOTRACK
  fi
  if ! iptables -w 60 -t raw -C OUTPUT -s "${KUBE_DNS_SVC_IP}/32" -p udp -m udp --sport 53 -j NOTRACK >/dev/null 2>&1; then
    iptables -w 60 -t raw -A OUTPUT -s "${KUBE_DNS_SVC_IP}/32" -p udp -m udp --sport 53 -j NOTRACK
  fi
  if iptables -w 60 -t raw -C PREROUTING -d "${KUBE_DNS_SVC_IP}/32" -m socket --nowildcard -j NOTRACK >/dev/null 2>&1 ; then
    # Remove. Will be added later.
    iptables -w 60 -t raw -D PREROUTING -d "${KUBE_DNS_SVC_IP}/32" -m socket --nowildcard -j NOTRACK >/dev/null 2>&1
  fi

  echo -n "not-ready" > /tmp/coredns-readiness
}

if [[ "$SHOULD_SETUP_IPTABLES" == "yes" ]]; then
  setup_iptables
fi

exec /coredns -conf /etc/coredns/Corefile
