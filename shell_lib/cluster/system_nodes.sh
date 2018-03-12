#!/bin/bash -e

function cluster::has_system_nodes() {
  if $(kubectl get nodes -l node-role/system -o name | grep . > /dev/null); then
    return 0
  else
    return 1
  fi
}

function cluster::count_system_nodes() {
  kubectl get nodes -l node-role/system -o name | wc -l
}
