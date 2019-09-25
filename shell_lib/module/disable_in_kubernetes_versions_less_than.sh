#!/bin/bash -e

function module::disable_in_kubernetes_versions_less_than() {
  cluster_version=$(values::get global.discovery.clusterVersion)
  if [ "$(semver compare $cluster_version $1)" -eq "-1" ] ; then
    echo "false" > $MODULE_ENABLED_RESULT
    exit 0
  fi
}
