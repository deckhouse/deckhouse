#!/bin/bash

config_values_json_patch=()
values_json_patch=()

function values::jq() {
  local values_path=$VALUES_PATH

  if [[ "$1" == "--config" ]] ; then
    values_path=$CONFIG_VALUES_PATH
    shift
  fi

  while true ; do
    case ${1:-} in
      --config)
        values_path=$CONFIG_VALUES_PATH
        shift
        ;;
      "")
        shift
        ;;
      *)
        break
        ;;
    esac
  done

  jq "${@}" "$values_path"
}

function values::get() {
  local required=no
  local config=""

  while true ; do
    case ${1:-} in
      --config)
        config="${1}"
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

  if [[ "$required" == "yes" ]] && ! values::has "$config" "${1:-}"; then
      >&2 echo "Error: Value $1 required, but doesn't exist"
      return 1
  fi

  jqPath="$(values::_convert_user_path_to_jq_path "${1:-}")"
  values::jq "$config" -r "$jqPath"
}

function values::set() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  values::_json_patch $config add $(values::_normalize_path_for_json_patch $1) "$2"
}

function values::has() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  local path=$(values::_dirname "${1:-}")
  local key=$(values::_basename "${1:-}")

  quotes='"'
  if [[ "$key" =~ ^[0-9]+$ ]]; then
    quotes=''
  fi

  jqPath="$(values::_convert_user_path_to_jq_path "${path}")"
  values::jq "$config" -e "${jqPath} | has(${quotes}${key}${quotes})" >/dev/null
}

function values::unset() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  if values::has $config $1 ; then
    values::_json_patch $config remove $(values::_normalize_path_for_json_patch $1)
  fi
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

  jqPath="$(values::_convert_user_path_to_jq_path "${1}")"
  values::jq "$config" -e "${jqPath}"' | (type == "array") and (index("'$2'") != null)' >/dev/null
}

function values::is_true() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  jqPath="$(values::_convert_user_path_to_jq_path "${1}")"
  values::jq "$config" -e "${jqPath} == true" >/dev/null
}

function values::is_false() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  jqPath="$(values::_convert_user_path_to_jq_path "${1}")"
  values::jq "$config" -e "${jqPath} == false" >/dev/null
}

function values::get_first_defined() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  for var in "$@"
  do
    if values::has "$config" "$var" ; then
      values::get "$config" "$var"
      return 0
    fi
  done
  return 1
}

function values::store::replace_row_by_key() {
  # [--config] <path> <key> <row>
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  KEY_VALUE=$(jq -rn --argjson row_values "$3" '$row_values | .'$2 )
  if INDEX=$(values::get $config $1 | jq -er 'to_entries[] | select(.value.'$2' == "'$KEY_VALUE'") | .key'); then
    values::_json_patch $config remove $(values::_normalize_path_for_json_patch $1)/$INDEX
    values::_json_patch $config add $(values::_normalize_path_for_json_patch $1)/$INDEX "$3"
  else
    values::_json_patch $config add $(values::_normalize_path_for_json_patch $1)/- "$3"
  fi
}

function values::store::unset_row_by_key() {
  # [--config] <path> <key> <row>
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  KEY_VALUE=$(jq -rn --argjson row_values "$3" '$row_values | .'$2 )
  if INDEX=$(values::get $config $1 | jq -er 'to_entries[] | select(.value.'$2' == "'$KEY_VALUE'") | .key'); then
    values::_json_patch $config remove $(values::_normalize_path_for_json_patch $1)/$INDEX
  fi
}

function values::_json_patch() {
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

function values::_normalize_path_for_json_patch() {
  # add a slash to the beginning
  # switch single-quote to double-quote
  # loop — hide dots in keys, i.e. aaa."bb.bb".ccc -> aaa."bb##DOT##bb".cc
  # delete double-quotes
  # switch dots to slashes
  # return original dots from ##DOT##
  sed -r \
    -e 's/^/\//' \
    -e s/\'/\"/g \
    -e ':loop' -e 's/"([^".]+)\.([^"]+)"/"\1##DOT##\2"/g' -e 't loop' \
    -e 's/"//g' \
    -e 's/\./\//g' \
    -e 's/##DOT##/./g' \
    <<< ${1}
}

function values::_convert_user_path_to_jq_path() {
  # change single-quote to double-quote (' -> ")
  # loop1 — hide dots in keys, i.e. aaa."bb.bb".ccc -> aaa."bb##DOT##bb".cc
  # loop2 — quote keys with symbol "-", i.e. 'myModule.internal.address-pool' -> 'myModule.internal."address-pool"'
  # loop3 — convert array addresation from myArray.0 to myArray[0]
  # loop4 — return original dots from ##DOT##, i.e. aaa."bb##DOT##bb".cc -> aa."bb.bb".cc

  jqPath=".$(sed -r \
    -e s/\'/\"/g \
    -e ':loop1' -e 's/"([^".]+)\.([^"]+)"/"\1##DOT##\2"/g' -e 't loop1' \
    -e ':loop2' -e 's/(^|\.)([^."]*-[^."]*)(\.|$)/\1"\2"\3/g' -e 't loop2' \
    -e ':loop3' -e 's/(^|\.)(\d+)(\.|$)/[\2]\3/g' -e 't loop3' \
    -e ':loop4' -e 's/(^|\.)"([^"]+)##DOT##([^"]+)"(\.|$)/\1"\2.\3"\4/g' -e 't loop4' \
    <<< "${1:-}"
  )"
  echo "${jqPath}"
}

function values::_dirname() {
  # loop1 — hide dots in keys, i.e. aaa."bb.bb".ccc -> aaa."bb##DOT##bb".cc
  splittable_path="$(sed -r -e s/\'/\"/g -e ':loop1' -e 's/"([^".]+)\.([^"]+)"/"\1##DOT##\2"/g' -e 't loop1' <<< ${1:-})"

  # loop2 — return original dots from ##DOT##, i.e. aaa."bb##DOT##bb".cc -> aa."bb.bb".cc
  rev <<< "${splittable_path}" | cut -d. -f2- | rev | sed -r -e ':loop2' -e 's/(^|\.)"([^"]+)##DOT##([^"]+)"(\.|$)/\1"\2.\3"\4/g' -e 't loop2'
}

function values::_basename() {
  # loop1 — hide dots in keys, i.e. aaa."bb.bb".ccc -> aaa."bb##DOT##bb".cc
  splittable_path="$(sed -r -e s/\'/\"/g -e ':loop1' -e 's/"([^".]+)\.([^"]+)"/"\1##DOT##\2"/g' -e 't loop1' <<< ${1:-})"

  # loop2 — return original dots from ##DOT##, i.e. "bb##DOT##bb" -> bb.bb
  rev <<< "${splittable_path}" | cut -d. -f1 | rev | sed -r -e ':loop2' -e 's/^"([^"]+)##DOT##([^"]+)"$/\1.\2/g' -e 't loop2'
}
