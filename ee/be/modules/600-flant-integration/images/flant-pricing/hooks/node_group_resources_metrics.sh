#!/bin/bash -e

# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat << EOF
    configVersion: v1
    kubernetes:
    - name: ngs
      group: ngs
      keepFullObjectsInMemory: false
      queue: /node_group_resources_metrics
      apiVersion: deckhouse.io/v1
      kind: NodeGroup
      jqFilter: {"name": .metadata.name, "labels": .spec.labels, "taints": .spec.taints}
    - name: nodes
      group: nodes
      keepFullObjectsInMemory: false
      queue: /node_group_resources_metrics
      apiVersion: deckhouse.io/v1
      kind: NodeGroup
      jqFilter: {"name": .metadata.name, labels: .metadata.labels, "cpu": .status.capacity.cpu, "memory": .status.capacity.memory}
EOF
}

function __main__() {
  group="group_node_group_resources_metrics"

  jq -c --arg group "$group" '.group = $group' <<< '{"action":"expire"}' >> $METRICS_PATH

  if ! context::has snapshots.ngs.0; then
    return 0
  fi

  if ! context::has snapshots.nodes.0; then
    return 0
  fi

  # "node.deckhouse.io/group": "worker",

  for node_group_name in $(context::jq -cr '.snapshots.ngs[].filterResult.name'); do
    cpu=0
    memory=0
    nodes=$(context::jq -cr --arg ng "$node_group_name" '.snapshots.nodes[].filterResult | select(.labels."node.deckhouse.io/group" == $ng) | .name')
    for node_name in $nodes; do
      node="$(context::jq -cr --arg node_name "$node_name" '.snapshots.nodes[].filterResult | select(.name == $node_name)')"
      node_cpu=$(jq -r '.cpu' <<< "$node")
      cpu=$(expr $cpu + $node_cpu)
      # Ki
      node_memory=$(jq -r '.memory' <<< "$node" | sed 's/[a-Z]//g')
      memory=$(expr $memory + $node_memory)
    done
  done

  node_group_labels
  context::jq -c --arg cpu "$cpu" --arg node_group_name "$node_group_name" --arg group "$group" '
    {
      "name": flant_pricing_node_group_cpu_cores,
      "group": $group,
      "set": $cpu,
      "labels": {
        "name": $node_group_name
      }
    }
    ' >> $METRICS_PATH
    context::jq -c --arg memory "$memory" --arg node_group_name "$node_group_name" --arg group "$group" '
      {
        "name": flant_pricing_node_group_memory_bytes,
        "group": $group,
        "set": $memory,
        "labels": {
          "name": $node_group_name
        }
      }
      ' >> $METRICS_PATH

}

hook::run "$@"
