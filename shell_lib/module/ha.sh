#!/bin/bash -e

function module::is_ha_enabled() {
  MODULE_NAME="$(module::name)"

  if values::has ${MODULE_NAME}.highAvailability; then
    flag=$(values::get ${MODULE_NAME}.highAvailability)
  elif values::has global.highAvailability; then
    flag=$(values::get global.highAvailability)
  else
    flag=$(values::get global.discovery.clusterControlPlaneIsHighlyAvailable)
  fi

  [[ $flag == "true" ]] # return flag
}
