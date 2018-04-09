#!/bin/bash

function hook::run() {
  if [[ "${1:-}" == "--config" ]] ; then
    __config__
  elif [[ -n "${CONFIG_VALUES_PATH:-}" && -n "${DYNAMIC_VALUES_PATH:-}" ]] ; then
    __main__
  else
    __main_old__
  fi
}
