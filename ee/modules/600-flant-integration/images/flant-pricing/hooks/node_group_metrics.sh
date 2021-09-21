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
    - name: ngs
      group: ngs
      keepFullObjectsInMemory: false
      queue: /node_group_metrics
      apiVersion: deckhouse.io/v1
      kind: NodeGroup
      jqFilter: if .status.nodes == .status.upToDate then 1 else 0 end
EOF
}

function __main__() {
  all_managed_nodes_up_to_date_metric_name="flant_pricing_all_managed_nodes_up_to_date"
  group="group_node_group_metrics"
  jq -c --arg group "$group" '.group = $group' <<< '{"action":"expire"}' >> $METRICS_PATH

  if ! context::has snapshots.ngs.0; then
    return 0
  fi

  context::jq -c --arg metric_name "$all_managed_nodes_up_to_date_metric_name" --arg group "$group" '
    {
      "name": $metric_name,
      "group": $group,
      "set": ([.snapshots.ngs[].filterResult] | sort | .[0])
    }
    ' >> $METRICS_PATH
}

hook::run "$@"
