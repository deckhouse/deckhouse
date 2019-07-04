#!/bin/bash

set -Eeo pipefail

chain_name="PREROUTING"
rule=(-d "$KUBE_DNS_SVC_IP/32" -m socket --nowildcard -j ACCEPT)

if [[ $1 == "--config" ]] ; then
  cat << EOF
{
  "onStartup": 10,
  "schedule": [
    {
      "name":"Every minute",
      "crontab":"*/1 * * * *"
    }
  ]
}
EOF
else
  iptables -w 600 -t nat -C "$chain_name" "${rule[@]}" >/dev/null 2>&1 || iptables -w 600 -t nat -I "$chain_name" 1 "${rule[@]}" >/dev/null 2>&1
fi
