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

if ! iptables -w 600 -C INPUT -m conntrack --ctstate INVALID -j DROP 2> /dev/null ; then
  iptables -w 600 -I INPUT 1 -m conntrack --ctstate INVALID -j DROP
fi

cp -f /etc/kube-flannel/cni-conf.json /etc/cni/net.d/10-flannel.conflist

# remove after 20.18 release
IPTABLES_SAVE="$(iptables-save -t nat | grep -vE "comment|docker0" | grep '^\-A POSTROUTING')"
echo -e "---\n${IPTABLES_SAVE}\n---"
echo "$IPTABLES_SAVE" | grep "^-A POSTROUTING ! -s .*/16 -d .*/16 -j MASQUERADE$" | awk '{print "iptables -t nat "$0}' | sed 's/-A/-D/' | bash
echo "$IPTABLES_SAVE" | grep "^-A POSTROUTING -s .*/16 ! -d .*/4 -j MASQUERADE$" | awk '{print "iptables -t nat "$0}' | sed 's/-A/-D/' | bash
echo -e "---\n$(iptables-save -t nat | grep -vE "comment|docker0" | grep '^\-A POSTROUTING')\n---"

exec /opt/bin/flanneld $@ $ifaces
