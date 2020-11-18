#!/bin/bash

function common_hooks::handle_host_ip_change::config() {
  namespace=$1
  name=$2
  cat <<EOF
    configVersion: v1
    kubernetes:
    - name: pods
      group: main
      keepFullObjectsInMemory: false
      executeHookOnEvent: ["Added", "Modified"]
      executeHookOnSynchronization: true
      apiVersion: v1
      kind: Pod
      namespace:
        nameSelector:
          matchNames: ["$namespace"]
      labelSelector:
        matchLabels:
          app: "$name"
      jqFilter: |
        {
          "name": .metadata.name,
          "hostIP": .status.hostIP,
          "initialHostIP": .metadata.annotations."node.deckhouse.io/initial-host-ip"
        }
EOF
}

function common_hooks::handle_host_ip_change::main() {
  namespace=$1
  if ! context::has snapshots.pods.0 ; then
    return 0
  fi

  pods=$(context::get snapshots.pods)
  for key in $(jq -rc 'keys[]' <<< "${pods}"); do
    pod=$(jq -rc --arg key "$key" '.[$key | tonumber]' <<< "${pods}")

    if jq -rce '.filterResult.hostIP | not' <<< "$pod"; then
      # Pod doesn't exist, we can skip it
      continue
    fi

    initial_host_ip=$(jq -rc '.filterResult.initialHostIP' <<< "$pod")

    if [[ "$initial_host_ip" == "null" ]]; then
      fltr=$(jq -rc '.filterResult | ".metadata.annotations.\"node.deckhouse.io/initial-host-ip\" = \"\(.hostIP)\""' <<< "$pod")
      kubernetes::patch_jq "$namespace" "pod/$(jq -rc '.filterResult.name' <<< "$pod")" "$fltr"
    elif [[ "$initial_host_ip" != $(jq -rc '.filterResult.hostIP' <<< "$pod") ]]; then
      kubernetes::delete_if_exists "$namespace" "pod/$(jq -rc '.filterResult.name' <<< "$pod")"
    fi
  done
}
