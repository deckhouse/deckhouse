#!/bin/bash -e

# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat << EOF
    configVersion: v1
    kubernetes:
    - name: nodes
      group: main
      queue: /node_metrics
      keepFullObjectsInMemory: false
      waitForSynchronization: false
      apiVersion: v1
      kind: Node
      labelSelector:
        matchExpressions:
        - key: "node.deckhouse.io/group"
          operator: Exists
      jqFilter: |
        select((.metadata.labels."node.deckhouse.io/group" == "master" and (.spec.taints == null or .spec.taints[].key != "node-role.kubernetes.io/control-plane")) or .metadata.labels."node.deckhouse.io/group" != "master") |
        {
          "nodeGroup": .metadata.labels."node.deckhouse.io/group",
          "pricingNodeType": (.metadata.annotations."pricing.flant.com/nodeType" // "unknown"),
          "virtualization": (.metadata.annotations."node.deckhouse.io/virtualization" // "unknown")
        }
    - name: ngs
      group: main
      queue: /node_metrics
      keepFullObjectsInMemory: false
      waitForSynchronization: false
      apiVersion: deckhouse.io/v1
      kind: NodeGroup
      jqFilter: |
        {
          "name": .metadata.name,
          "nodeType": .spec.nodeType
        }
EOF
}

function __main__() {
  count_nodes_by_type_metric_name="flant_pricing_count_nodes_by_type"
  group="group_node_metrics"
  jq -c --arg group "$group" '.group = $group' <<< '{"action":"expire"}' >> $METRICS_PATH

  node_types=( ephemeral vm hard special )
  ephemeral_nodes=0
  vm_nodes=0
  hard_nodes=0
  special_nodes=0

  for node_index in $(context::jq '.snapshots.nodes | to_entries[] | select(.value.filterResult.nodeGroup != null) | .key'); do
    node_data="$(context::get snapshots.nodes.$node_index.filterResult)"
    node_group="$(jq -r '.nodeGroup' <<< "$node_data")"
    pricing_node_type="$(jq -r '.pricingNodeType' <<< "$node_data")"
    virtualization="$(jq -r '.virtualization' <<< "$node_data")"

    if ! ng_data="$(context::jq -erc --arg node_group "$node_group" '.snapshots.ngs[] | select(.filterResult.name == $node_group) | .filterResult')"; then
      pricing="$pricing_node_type"
    else
      pricing="$(
        jq -nr --arg pricing_node_type "$pricing_node_type" \
          --arg virtualization "$virtualization" \
          --argjson ng "$ng_data" '
          if $pricing_node_type != "unknown"
            then $pricing_node_type
          else
            if $ng.nodeType == "CloudEphemeral" then "Ephemeral"
            elif $ng.nodeType == "CloudPermanent" or $ng.nodeType == "CloudStatic" then "VM"
            elif $ng.nodeType == "Static" and $virtualization != "unknown" then "VM"
            else "Hard"
            end
          end
      ')"
    fi

    case $pricing in
      Ephemeral)
        (( ephemeral_nodes = ephemeral_nodes + 1 ))
      ;;
      VM)
        (( vm_nodes = vm_nodes + 1 ))
      ;;
      Hard)
        (( hard_nodes = hard_nodes + 1 ))
      ;;
      Special)
        (( special_nodes = special_nodes + 1 ))
      ;;
    esac
  done

  for node_type in "${node_types[@]}"; do
    count_variable_name="${node_type}_nodes"
    jq -nc --arg metric_name $count_nodes_by_type_metric_name --arg group "$group" \
      --arg node_type "$node_type" \
      --argjson count "${!count_variable_name}" '
      {
        "name": $metric_name,
        "group": $group,
        "set": $count,
        "labels": {
          "type": $node_type
        }
      }
      ' >> $METRICS_PATH
  done
}

hook::run "$@"
