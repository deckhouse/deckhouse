#!/bin/bash -e

# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat << EOF
    configVersion: v1
    onStartup: 20
EOF
}

# Output metric if the value is not empty.
# $1 - metric name
# $2 - metric value
function output_metric() {
  if [[ "$2" == "" ]]; then
    >&2 echo "ERROR: Skipping empty value metric $1."
    return 0
  fi

  jq -n --arg name "$1" --argjson value "$2" '
    {
      "name": $name,
      "set": $value
    }
    ' >> $METRICS_PATH
}

function __main__() {
  echo '
  {
    "name": "flant_pricing_cluster_info",
    "set": '$(date +%s)',
    "labels": {
      "release_channel": "'$FP_RELEASE_CHANNEL'",
      "bundle": "'$FP_BUNDLE'",
      "cloud_provider": "'$FP_CLOUD_PROVIDER'",
      "cloud_layout": "'$FP_CLOUD_LAYOUT'",
      "control_plane_version": "'$FP_CONTROL_PLANE_VERSION'",
      "minimal_kubelet_version": "'$FP_MINIMAL_KUBELET_VERSION'",
      "deckhouse_version": "'$FP_DECKHOUSE_VERSION'",
      "pricing_cluster_type": "'$FP_CLUSTER_TYPE'"
    }
  }' | jq -rc >> $METRICS_PATH

  output_metric "flant_pricing_masters_count" "$FP_MASTERS_COUNT"
  output_metric "flant_pricing_master_is_dedicated" "$FP_MASTER_IS_DEDICATED"
  output_metric "flant_pricing_master_min_cpu" "$FP_MASTER_MIN_CPU"
  output_metric "flant_pricing_master_min_memory" "$FP_MASTER_MIN_MEMORY"
  output_metric "flant_pricing_plan_is_bought_as_bundle" "$FP_PLAN_IS_BOUGHT_AS_BUNDLE"
  output_metric "flant_pricing_do_not_charge_for_rock_solid" "$FP_DO_NOT_CHARGE_FOR_ROCK_SOLID"
  output_metric "flant_pricing_contacts" "$FP_CONTACTS"
  output_metric "flant_pricing_auxiliary_cluster" "$FP_AUXILIARY_CLUSTER"
  output_metric "flant_pricing_nodes_discount" "$FP_NODES_DISCOUNT"

  if [[ "$FP_KUBEALL_HOST" != "" ]]; then
    echo '
    {
      "name": "flant_pricing_kubeall",
      "set": '$(date +%s)',
      "labels": {
        "host": "'$FP_KUBEALL_HOST'",
        "kubectl": "'$FP_KUBEALL_KUBECTL'",
        "kubeconfig": "'$FP_KUBEALL_KUBECONFIG'",
        "context": "'$FP_KUBEALL_CONTEXT'"
      }
    }' | jq -rc >> $METRICS_PATH
  fi
}

hook::run "$@"
