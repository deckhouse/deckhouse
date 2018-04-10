#!/bin/bash

config_values_json_patch=()
dynamic_values_json_patch=()

function values::json_patch() {
  if [[ "$1" == "--config" ]] ; then
    shift
    config_values_json_patch+=($(jo $@))
    printf '%s\n' "${config_values_json_patch[@]}" | jo -a > $CONFIG_VALUES_JSON_PATCH_PATH
  else
    dynamic_values_json_patch+=($(jo $@))
    printf '%s\n' "${dynamic_values_json_patch[@]}" | jo -a > $DYNAMIC_VALUES_JSON_PATCH_PATH
  fi
}

function values::get() {
  if [[ "$1" == "--config" ]] ; then
    shift
    cat $CONFIG_VALUES_PATH | jq ".$1" -r
  else
    cat $DYNAMIC_VALUES_PATH | jq ".$1" -r
  fi
}

function values::set() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  values::json_patch $config op=add path=/$(echo $1 | sed 's/\./\//g') value="$2"
}

function values::has() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  local path=.$(echo $1 | rev | cut -d. -f2- | rev)
  local key=$(echo $1 | rev | cut -d. -f1 | rev)
    
  if [[ "$(values::get $config | jq $path' | has("'$key'")' -r)" == "true" ]] ; then
    return 0
  else
    return 1
  fi
}

function values::unset() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  if values::has $config $1 ; then
    values::json_patch $config op=remove path=/$(echo $1 | 'sed s/\./\//g')
  fi
}

function values::is_empty() {
  [[ -z "$1" || "$1" == "null" ]]
}
