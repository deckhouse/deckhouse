#!/bin/bash -e

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat << EOF
    configVersion: v1
    onStartup: 20
EOF
}

function __main__() {
  echo '
  {
    "name": "flant_pricing_cluster_info",
    "set": 1,
    "labels": {
      "project": "'$FP_PROJECT'",
      "cluster": "'$FP_CLUSTER'",
      "release_channel": "'$FP_RELEASE_CHANNEL'",
      "bundle": "'$FP_BUNDLE'",
      "cloud_provider": "'$FP_CLOUD_PROVIDER'",
      "control_plane_version": "'$FP_CONTROL_PLANE_VERSION'",
      "minimal_kubelet_version": "'$FP_MINIMAL_KUBELET_VERSION'",
      "pricing_plan": "'$FP_PLAN'",
      "pricing_cluster_type": "'$FP_CLUSTER_TYPE'"
    }
  }' | jq -rc >> $METRICS_PATH
  echo '{"name":"flant_pricing_masters_count","set":'"$FP_MASTERS_COUNT"'}' >> $METRICS_PATH
  echo '{"name":"flant_pricing_kops","set":'"$FP_KOPS"'}' >> $METRICS_PATH
  echo '{"name":"flant_pricing_all_managed_nodes_up_to_date","set":'"$FP_ALL_MANAGED_NODES_UP_TO_DATE"'}' >> $METRICS_PATH
  echo '{"name":"flant_pricing_converge_is_completed","set":'"$FP_CONVERGE_IS_COMPLETED"'}' >> $METRICS_PATH
  echo '{"name":"flant_pricing_deprecated_resources_in_helm_releases","set":'"$FP_DEPRECATED_RESOURCES_IN_HELM_RELEASES"'}' >> $METRICS_PATH
  echo '{"name":"flant_pricing_master_is_dedicated","set":'"$FP_MASTER_IS_DEDICATED"'}' >> $METRICS_PATH
  echo '{"name":"flant_pricing_master_min_cpu","set":'"$FP_MASTER_MIN_CPU"'}' >> $METRICS_PATH
  echo '{"name":"flant_pricing_master_min_memory","set":'"$FP_MASTER_MIN_MEMORY"'}' >> $METRICS_PATH
  echo '{"name":"flant_pricing_plan_is_bought_as_bundle","set":'"$FP_PLAN_IS_BOUGHT_AS_BUNDLE"'}' >> $METRICS_PATH
  echo '{"name":"flant_pricing_do_not_charge_for_rock_solid","set":'"$FP_DO_NOT_CHARGE_FOR_ROCK_SOLID"'}' >> $METRICS_PATH
  echo '{"name":"flant_pricing_contacts","set":'"$FP_CONTACTS"'}' >> $METRICS_PATH
}

hook::run "$@"
