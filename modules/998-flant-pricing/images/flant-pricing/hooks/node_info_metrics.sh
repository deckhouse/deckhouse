#!/bin/bash -e

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat << EOF
    configVersion: v1
    kubernetes:
    - name: nodes
      group: node_info
      keepFullObjectsInMemory: false
      waitForSynchronization: false
      apiVersion: v1
      kind: Node
      labelSelector:
        matchExpressions:
        - key: "node.deckhouse.io/group"
          operator: Exists
      jqFilter: |
        select((.metadata.labels."node.deckhouse.io/group" == "master" and (.spec.taints == null or .spec.taints[].key != "node-role.kubernetes.io/master")) or .metadata.labels."node.deckhouse.io/group" != "master") |
        {
          "name": .metadata.name,
          "nodeGroup": .metadata.labels."node.deckhouse.io/group",
          "pricingNodeType": (.metadata.annotations."pricing.flant.com/nodeType" // "Unknown")
        }
    - name: ngs
      group: node_info
      keepFullObjectsInMemory: false
      waitForSynchronization: false
      apiVersion: deckhouse.io/v1alpha1
      kind: NodeGroup
      jqFilter: |
        {
          "name": .metadata.name,
          "nodeType": .spec.nodeType
        }
EOF
}


function __on_group::node_info() {
  for node_index in $(context::jq '.snapshots.nodes | to_entries[] | select(.value.filterResult.nodeGroup != null) | .key'); do
    node_data="$(context::get snapshots.nodes.$node_index.filterResult)"
    node_name="$(jq -r '.name' <<< "$node_data")"
    node_group="$(jq -r '.nodeGroup' <<< "$node_data")"
    pricing_node_type="$(jq -r '.pricingNodeType' <<< "$node_data")"

    if ! ng_data="$(context::jq -er --arg node_group "$node_group" '.snapshots.ngs[] | select(.filterResult.name == $node_group) | .filterResult')"; then
      pricing="$pricing_node_type"
    else
      pricing="$(
        jq -nr --arg pricing_node_type "$pricing_node_type" --argjson ng "$ng_data" '
          if $pricing_node_type == "Unknown" and $ng.nodeType == "Cloud" then "Ephemeral"
          elif $pricing_node_type == "Unknown" and $ng.nodeType == "Hybrid" then "VM"
          else $pricing_node_type
          end
      ')"
    fi

    metric_name="flant_pricing_node_info"
    jq -nc --arg metric_name $metric_name --arg node_name "$node_name" --arg pricing "$pricing" '
        {"name": $metric_name, "group": "/modules/'$(module::name::kebab_case)'/metrics#'$metric_name'", "set": 1, "labels": {"node": $node_name, "pricing": $pricing}}
        ' >> $METRICS_PATH
  done
}

hook::run "$@"
