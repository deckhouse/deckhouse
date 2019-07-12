#!/bin/bash -e

function cluster::count_nodes_by_role() {
  if values::has global.discovery.nodeCountByRole."$1"; then
    values::get global.discovery.nodeCountByRole."$1"
  else
    echo -n "0"
  fi
}
