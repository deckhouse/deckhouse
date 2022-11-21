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

      # We do not charge for control plane nodes which are in desired state
      #   1. we check the node is NOT control plane node group, so we charge for it
      #   OR
      #   2. the node is from control plane node group, BUT has no expected taints
      #      meaning they were reconfigured by user

      jqFilter: |
        select(
          .metadata.labels."node.deckhouse.io/group" != "master"
          or
          (
            .spec.taints == null
            or
            (
              [
                .spec.taints[]
                | select(
                  .key == "node-role.kubernetes.io/control-plane" or
                  .key == "node-role.kubernetes.io/master"
                )
              ]
              | length == 0
            )
          )
        )
        | {
          "nodeGroup":        .metadata.labels."node.deckhouse.io/group",
          "pricingNodeType": (.metadata.annotations."pricing.flant.com/nodeType"       // "unknown"),
          "virtualization":  (.metadata.annotations."node.deckhouse.io/virtualization" // "unknown")
        }

    - name: nodes_all
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

      # All nodes managed by deckhouse, counting into flant_pricing_node_count
      jqFilter: |
        select( .metadata.labels."node.deckhouse.io/group" != null )
        | {
          "nodeGroup":        .metadata.labels."node.deckhouse.io/group",
          "pricingNodeType": (.metadata.annotations."pricing.flant.com/nodeType"       // "unknown"),
          "virtualization":  (.metadata.annotations."node.deckhouse.io/virtualization" // "unknown")
        }

    - name: nodes_cp
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

      # Control plane nodes, counting into flant_pricing_controlplane_node_count
      jqFilter: |
        select( .metadata.labels."node.deckhouse.io/group" == "master" )
        | {
          "nodeGroup":        .metadata.labels."node.deckhouse.io/group",
          "pricingNodeType": (.metadata.annotations."pricing.flant.com/nodeType"       // "unknown"),
          "virtualization":  (.metadata.annotations."node.deckhouse.io/virtualization" // "unknown")
        }

    - name: nodes_t_cp
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

      # Control plane nodes with desired taints, counting into flant_pricing_controlplane_tainted_node_count
      jqFilter: |
        select(
          .metadata.labels."node.deckhouse.io/group" == "master"
          and
          .spec.taints != null
          and
          (
            [
              .spec.taints[]
              | select(
                .key == "node-role.kubernetes.io/control-plane" or
                .key == "node-role.kubernetes.io/master"
              )
            ]
            | length > 0
          )
        )
        | {
          "nodeGroup":        .metadata.labels."node.deckhouse.io/group",
          "pricingNodeType": (.metadata.annotations."pricing.flant.com/nodeType"       // "unknown"),
          "virtualization":  (.metadata.annotations."node.deckhouse.io/virtualization" // "unknown")
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
  group="group_node_metrics"
  jq -c --arg group "$group" '.group = $group' <<< '{"action":"expire"}' >> $METRICS_PATH

  generate_node_count_metric "nodes"      "$group" "flant_pricing_count_nodes_by_type"        # DEPRECATED all nodes except CP nodes with expected taints
  generate_node_count_metric "nodes_all"  "$group" "flant_pricing_nodes"                      # all nodes
  generate_node_count_metric "nodes_cp"   "$group" "flant_pricing_controlplane_nodes"         # CP nodes
  generate_node_count_metric "nodes_t_cp" "$group" "flant_pricing_controlplane_tainted_nodes" # CP nodes with expected taints
}

# Args:
#   $1 - snapshot name
#   $2 - metric group
#   $3 - metric name
function generate_node_count_metric() {
  local snapshot_name="$1"
  local metric_group="$2"
  local metric_name="$3"

  metric_node_types=( ephemeral vm hard special )
  ephemeral_nodes=0
  vm_nodes=0
  hard_nodes=0
  special_nodes=0

  for node_index in $(context::jq --arg snapshot_name "${snapshot_name}" '.snapshots[$snapshot_name] | to_entries[] | select(.value.filterResult.nodeGroup != null) | .key '); do
    node_data="$(context::get snapshots.${snapshot_name}.${node_index}.filterResult)"
    node_group="$(     jq -r '.nodeGroup'       <<< "$node_data" )"
    type_from_node="$( jq -r '.pricingNodeType' <<< "$node_data" )"
    virtualization="$( jq -r '.virtualization'  <<< "$node_data" )"

    if ! ng_data="$(context::jq -erc --arg node_group "$node_group" '.snapshots.ngs[] | select(.filterResult.name == $node_group) | .filterResult')"; then
      # NodeGroup snapshot is not found, use pricing node type from the node
      pricing_node_type="$type_from_node"
    else
      # NodeGroup snapshot is found, try to get pricing node type from node and fall back to getting it from NodeGroup
      pricing_node_type="$(
        jq -nr \
          --arg type_from_node "$type_from_node" \
          --arg virtualization "$virtualization" \
          --argjson ng "$ng_data" '
          if $type_from_node != "unknown"
            then $type_from_node
          else
            if   $ng.nodeType == "CloudEphemeral"                                   then "Ephemeral"
            elif $ng.nodeType == "CloudPermanent" or $ng.nodeType == "CloudStatic"  then "VM"
            elif $ng.nodeType == "Static" and $virtualization != "unknown"          then "VM"
            else "Hard"
            end
          end
      ')"
    fi

    case $pricing_node_type in
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

  for node_type in "${metric_node_types[@]}"; do
    count_variable_name="${node_type}_nodes"
    jq -nc \
      --arg metric_group "$metric_group" \
      --arg metric_name  "$metric_name"  \
      --arg node_type    "$node_type"    \
      --argjson count "${!count_variable_name}" '
      {
        "name": $metric_name,
        "group": $metric_group,
        "set": $count,
        "labels": {
          "type": $node_type
        }
      }
      ' >>$METRICS_PATH
  done
}

hook::run "$@"
