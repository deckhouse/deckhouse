#!/bin/bash

set -Eeo pipefail

lockfile-create /tmp/lock

if [[ -f /tmp/shutting_down ]] ; then
  exit 0
fi

curl -sS "127.0.0.1:9225/health" >/dev/null

# Check internal cluster DNS name
dig kubernetes.default.svc.${KUBE_CLUSTER_DOMAIN}. @169.254.20.10 +short +timeout=2 +tries=2 | grep -v -e '^$' >/dev/null

if ! iptables -w 600 -t raw -C PREROUTING -d ${KUBE_DNS_SVC_IP}/32 -m socket --nowildcard -j NOTRACK >/dev/null 2>&1 ; then
  iptables -w 600 -t raw -A PREROUTING -d ${KUBE_DNS_SVC_IP}/32 -m socket --nowildcard -j NOTRACK
fi

lockfile-remove /tmp/lock