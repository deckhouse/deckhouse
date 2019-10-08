#!/bin/bash

set -Eeo pipefail

function enable_traffic() {
  if ! iptables -w 600 -t raw -C PREROUTING -d ${KUBE_DNS_SVC_IP}/32 -m socket --nowildcard -j NOTRACK >/dev/null 2>&1 ; then
    iptables -w 600 -t raw -A PREROUTING -d ${KUBE_DNS_SVC_IP}/32 -m socket --nowildcard -j NOTRACK
  fi
}

function disable_traffic() {
  if iptables -w 600 -t raw -C PREROUTING -d ${KUBE_DNS_SVC_IP}/32 -m socket --nowildcard -j NOTRACK >/dev/null 2>&1 ; then
    iptables -w 600 -t raw -D PREROUTING -d ${KUBE_DNS_SVC_IP}/32 -m socket --nowildcard -j NOTRACK
  fi
}

function dns_ready() {
  set -Eeuo pipefail
  # `dig` returns non-zero exit code only when there is a server failure (SERVFAIL),
  # it won't return non-zero exit code on NXDOMAIN.
  # Here we generate a random, certain-to-not-be-in-cache DNS request.
  dig "$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c10).default.svc.${KUBE_CLUSTER_DOMAIN}." @169.254.20.10 +short +timeout=2 +tries=2 >/dev/null

  # Check internal cluster DNS name
  dig kubernetes.default.svc.${KUBE_CLUSTER_DOMAIN}. @169.254.20.10 +short +timeout=2 +tries=2 | grep -v -e '^$' >/dev/null

  # Check external DNS name
  dig google.com @169.254.20.10 +short +timeout=2 +tries=2 | grep -v -e '^$' >/dev/null

  curl -sS "127.0.0.1:9225/health" >/dev/null
}

if [[ $1 == "--config" ]] ; then
  cat << EOF
{
  "schedule": [
    {
      "name":"Every 5 second",
      "crontab":"*/5 * * * * *"
    }
  ]
}
EOF
else
  touch /alive
  touch /ready

  if [[ -f "/shared/starting" ]] ; then
    disable_traffic
    touch /shared/dns_alive
    rm /shared/starting
  fi

  if nc -z 169.254.20.10 53 ; then
    touch /shared/dns_alive

    if dns_ready; then
      enable_traffic
      touch /shared/dns_ready
    else
      disable_traffic
    fi
  else
    disable_traffic
  fi
fi
