#!/bin/bash
set -Eeuo pipefail

node_object="$(curl -sS -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" -k https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}/api/v1/nodes/$(hostname))"
nodeport_bind_internal_ip="$(jq -re '.metadata.annotations."node.deckhouse.io/nodeport-bind-internal-ip" // true' <<< "$node_object")"

internalip="$(jq -re '[.status.addresses[] | select(.type == "InternalIP").address] | (first | "\(.)/32") // ""' <<< "$node_object")"

if [ -z "$internalip" ]; then
  >&2 echo "ERROR: Node $(hostname) doesn't have InternalIP in .status.addresses"
  exit 1
fi

if [ "$nodeport_bind_internal_ip" == "false" ]; then
  internalip="0.0.0.0/0"
fi

sed "s#__node_address__#$internalip#" /var/lib/kube-proxy-cm/config.conf > /var/lib/kube-proxy/config.conf
