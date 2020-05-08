#!/bin/bash

status=$(curl -s -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" -k https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}/api/v1/nodes/$(hostname)/status | jq -r '.status.addresses[]')
internalips=$(jq -r 'select(.type == "InternalIP") | .address' <<< "$status")
externalips=$(jq -r 'select(.type == "ExternalIP") | .address' <<< "$status")

ifaces=""
for ip in $internalips $externalips; do
  ifaces="$ifaces -iface $ip"
done

if [ ! -n "$ifaces" ]; then
  >&2 echo "ERROR: Node $(hostname) doesn't have neither InternalIP nor ExternalIP in .status.addresses"
  exit 1
fi

exec /opt/bin/flanneld $@ $ifaces
