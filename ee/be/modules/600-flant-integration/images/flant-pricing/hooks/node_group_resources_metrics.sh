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
      jqFilter: |
        {
          "name": .metadata.name,
          "labels": .spec.labels,
          "taints": .spec.taints
        }
EOF
}

function __main__() {
  ngs_capacity="$FP_NODE_GROUPS_CAPACITY"
  group="group_node_group_resources_metrics"

  jq -c --arg group "$group" '.group = $group' <<< '{"action":"expire"}' >> $METRICS_PATH

  if ! context::has snapshots.ngs.0; then
    return 0
  fi

  if [ "$ngs_capacity" == "" ]; then
    return 0
  fi

  for node_group_name in $(context::jq -cr '.snapshots.ngs[].filterResult.name'); do
    ng_capacity="$(jq -cr --arg ng_name "$node_group_name" '.[$ng_name] // ""' <<< "$ngs_capacity")"
    if [ "$ng_capacity" == "" ]; then
      continue
    fi
    cpu="$(jq -cr '.CPU' <<< "$ng_capacity")"
    memory="$(jq -cr '.memory' <<< "$ng_capacity")"
    labels="$(context::jq -cr --arg ng_name "$node_group_name" '.snapshots.ngs[].filterResult | select(.name == $ng_name) | .labels // {} * {"name": .name}')"

    context::jq -c --arg cpu "$cpu" --argjson labels "$labels" --arg group "$group" '
      {
        "name": "flant_pricing_node_group_cpu_cores",
        "group": $group,
        "set": $cpu,
        "labels": $labels
      }
      ' >> $METRICS_PATH
    context::jq -c --arg memory "$memory" --argjson labels "$labels" --arg group "$group" '
      {
        "name": "flant_pricing_node_group_memory_bytes",
        "group": $group,
        "set": $memory,
        "labels": $labels
      }
      ' >> $METRICS_PATH
  done

}

hook::run "$@"
