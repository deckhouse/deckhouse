#!/bin/bash

set -Eeo pipefail

lockfile-create /tmp/lock

touch /tmp/shutting_down

if iptables -w 600 -t raw -C PREROUTING -d ${KUBE_DNS_SVC_IP}/32 -m socket --nowildcard -j NOTRACK >/dev/null 2>&1 ; then
  iptables -w 600 -t raw -D PREROUTING -d ${KUBE_DNS_SVC_IP}/32 -m socket --nowildcard -j NOTRACK >/dev/null 2>&1
fi

killall coredns

lockfile-remove /tmp/lock
