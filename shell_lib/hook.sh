#!/bin/bash

function hook::run() {
  if [[ "${1:-}" == "--config" ]] ; then
    __config__
  else
    __main__
  # Костыль 2019-10-15
  # Удалить после дебага
  set +u
  if [[ -n $VALUES_JSON_PATCH_PATH ]] ; then
    mkdir -p /tmp/$(dirname $0)
    echo "# "$(date) >> /tmp/$0
    cat $VALUES_JSON_PATCH_PATH >> /tmp/$0
  fi
  fi
}
