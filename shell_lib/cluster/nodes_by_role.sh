#!/bin/bash -e

function cluster::count_nodes_by_role() {
  if count=$(values::get --required global.discovery.nodeCountByRole."$1"); then
    echo -n "$count"
  else
    echo -n "0"
  fi
}
