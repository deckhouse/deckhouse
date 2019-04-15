#!/bin/bash

config_values_json_patch=()
values_json_patch=()

function values::json_patch() {
  set -f
  if [[ "$1" == "--config" ]] ; then
    shift
    config_values_json_patch+=($(jq -nec --arg op "$1" --arg path "$2" --arg value "${3:-""}" \
                                '{"op": $op, "path": $path} + if (($value | length) > 0) then {"value": (try ($value | fromjson) catch $value)} else {} end'))

    echo "${config_values_json_patch[@]}" | \
      jq -sec '.' > $CONFIG_VALUES_JSON_PATCH_PATH
  else
    values_json_patch+=($(jq -nec --arg op "$1" --arg path "$2" --arg value "${3:-""}" \
                                '{"op": $op, "path": $path} + if (($value | length) > 0) then {"value": (try ($value | fromjson) catch $value)} else {} end'))

    echo "${values_json_patch[@]}" | \
      jq -sec '.' > $VALUES_JSON_PATCH_PATH
  fi
  set +f
}

function values::get() {
  local values_path=$VALUES_PATH
  local required=no

  while true ; do
    case ${1:-} in
      --config)
        values_path=$CONFIG_VALUES_PATH
        shift
        ;;
      --required)
        required=yes
        shift
        ;;
      *)
        break
        ;;
    esac
  done

  local value=$(cat $values_path | jq ".${1:-}" -r)

  if [[ "$required" == "yes" ]] && values::is_empty "$value" ; then
    >&2 echo "Error: Value $1 required, but empty"
    return 1
  else
    echo $value
    return 0
  fi
}

function values::set() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  values::json_patch $config add /$(echo $1 | sed 's/\./\//g') "$2"
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
    values::json_patch $config remove /$(echo $1 | sed 's/\./\//g')
  fi
}

function values::is_empty() {
  [[ -z "${1:-}" || "${1:-}" == "null" ]]
}

function values::require_in_config() {
  if ! values::has --config $1 ; then
    >&2 echo "Error: $1 is required in config!"
    return 1
  fi
}

function values::array_has() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  values::get $config $1 | jq '(type == "array") and (index("'$2'") != null)' -e > /dev/null
}

function values::is_true() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  values::get $config $1 | jq '. == true' -e > /dev/null
}

function values::is_false() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  values::get $config $1 | jq '. == false' -e > /dev/null
}

function values::generate_password() {
  makepasswd -c 0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ -l 20
}

function values::get_first_defined() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  for var in "$@"
  do
    if values::has $config $var ; then
      values::get $config $var
    fi
    return
  done
}
