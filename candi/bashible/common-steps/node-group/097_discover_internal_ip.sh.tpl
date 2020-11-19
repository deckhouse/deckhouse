{{- if ne .runType "ClusterBootstrap" }}
  {{- if eq .nodeGroup.nodeType "Static" }}

# Migration: delete it in release 20.19
if addr=$(kubectl --kubeconfig=/etc/kubernetes/kubelet.conf get nodes $(hostname -s) -o json \
  | jq -rc '[.status.addresses[] | select(.type == "InternalIP")| .address][0]'); then

  if [[ "$addr" == "null" ]]; then
    exit 0
  fi

  address_with_mask=$(ip -4 a | grep -oh "$addr/[0-9]*")
  kubectl --kubeconfig=/etc/kubernetes/kubelet.conf annotate node $(hostname -s) --overwrite \
    "node.deckhouse.io/internal-network-cidr=$address_with_mask"
fi
  {{- end }}
{{- end }}
