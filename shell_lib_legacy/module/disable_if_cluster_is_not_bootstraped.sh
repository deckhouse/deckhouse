#!/bin/bash -e

function module::disable_if_cluster_is_not_bootstraped() {
  if ! values::is_true global.clusterIsBootstrapped ; then
    echo "false" > $MODULE_ENABLED_RESULT
    exit 0
  fi
}
