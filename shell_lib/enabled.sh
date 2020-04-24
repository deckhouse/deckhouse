#!/bin/bash

function enabled::run() {
    __main__
}

function enabled::disable_module_if_cluster_is_not_bootstraped() {
  if ! values::is_true global.clusterIsBootstrapped ; then
    echo "false" > $MODULE_ENABLED_RESULT
    exit 0
  fi
}

function enabled::disable_module_in_kubernetes_versions_less_than() {
  cluster_version=$(values::get global.discovery.kubernetesVersion)
  if [ "$(semver compare $cluster_version $1)" -eq "-1" ] ; then
    echo "false" > $MODULE_ENABLED_RESULT
    exit 0
  fi
}
