# Ensure we have file
touch /var/lib/bashible/discovered-node-ip

{{- if ne .nodeGroup.nodeType "Static" }}
# For Cloud or Hybrid node we try to discover IP from Node object

if [ -f /etc/kubernetes/kubelet.conf ] ; then
  if node="$(kubectl --kubeconfig=/etc/kubernetes/kubelet.conf get node $HOSTNAME -o json 2> /dev/null)" ; then
    echo "$node" | jq -r '([.status.addresses[] | select(.type == "InternalIP") | .address] + [.status.addresses[] | select(.type == "ExternalIP") | .address])[0] // ""' > /var/lib/bashible/discovered-node-ip
  else
    bb-log-error "Unable to discover node IP for node object: No access to API server"
    exit 1
  fi
fi
{{- end }}

{{- if and (eq .nodeGroup.nodeType "Static") (hasKey .nodeGroup "static") }}
  {{- if not (hasKey .nodeGroup.static "internalNetworkCIDRs") }}
# No .nodeGroup.static.internalNetworkCIDRs in Static node
echo "" > /var/lib/bashible/discovered-node-ip
  {{- else }}
# For Static node we use .nodeGroup.static.internalNetworkCIDRs

function is_ip_in_cidr() {
  ip="$1"
  IFS="/" read net_address net_prefix <<< "$2"

  IFS=. read -r a b c d <<< "$ip"
  ip_dec="$((a * 256 ** 3 + b * 256 ** 2 + c * 256 + d))"

  IFS=. read -r a b c d <<< "$net_address"
  net_address_dec="$((a * 256 ** 3 + b * 256 ** 2 + c * 256 + d))"

  netmask=$(((0xFFFFFFFF << (32 - net_prefix)) & 0xFFFFFFFF))

  test $((netmask & ip_dec)) -eq $((netmask & net_address_dec))
}

ip_in_system=$(ip -f inet -br -j addr | jq -r '.[] | .addr_info[] | .local')

for cidr in {{ .nodeGroup.static.internalNetworkCIDRs | join " " }}; do
  for ip in $ip_in_system; do
    if is_ip_in_cidr "$ip" "$cidr"; then
      echo $ip > /var/lib/bashible/discovered-node-ip
      exit 0
    fi
  done
done
  {{- end }}

  {{- if eq .runType "ClusterBootstrap" }}
if [ -z "$(cat /var/lib/bashible/discovered-node-ip)" ] ; then
  bb-log-error "Failed to discover node_ip but it's required for cluster bootstrap"
  exit 1
fi
  {{- end }}
{{- end }}
