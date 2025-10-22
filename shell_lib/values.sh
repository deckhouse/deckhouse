#!/bin/bash

# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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

  if [[ "$required" == "yes" ]] && ! values::has $config "${1:-}"; then
      >&2 echo "Error: Value $1 required, but doesn't exist"
      return 1
  fi

  jqPath="$(context::_convert_user_path_to_jq_path "${1:-}")"
  values::jq "$config" -r "$jqPath"
}

function values::set() {
  local config=""
  local values_path=$VALUES_PATH
  if [[ "$1" == "--config" ]] ; then
    values_path=$CONFIG_VALUES_PATH
    config=$1
    shift
  fi

  normalized_path_for_json_patch="$(values::_normalize_path_for_json_patch "$1")"
  normalized_path_for_jq="$(context::_convert_user_path_to_jq_path "$1")"

  values::_json_patch $config add "${normalized_path_for_json_patch}" "$2"
  patched_values="$(jq --arg value "${2}" "${normalized_path_for_jq}"' = (try ($value | fromjson) catch $value)' ${values_path})"
  echo "${patched_values}" > $values_path
}

function values::has() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  local path=$(context::_dirname "${1:-}")
  local key=$(context::_basename "${1:-}")

  quotes='"'
  if [[ "$key" =~ ^[0-9]+$ ]]; then
    quotes=''
  fi

  jqPath="$(context::_convert_user_path_to_jq_path "${path}")"
  values::jq "$config" -e "${jqPath} | has(${quotes}${key}${quotes})" >/dev/null
}

function values::unset() {
  local config=""
  local values_path=$VALUES_PATH
  if [[ "$1" == "--config" ]] ; then
    config=$1
    values_path=$CONFIG_VALUES_PATH
    shift
  fi

  if values::has $config $1 ; then
    normalized_path_for_json_patch="$(values::_normalize_path_for_json_patch $1)"
    normalized_path_for_jq="$(context::_convert_user_path_to_jq_path $1)"

    values::_json_patch $config remove "${normalized_path_for_json_patch}"
    patched_values="$(jq "del(${normalized_path_for_jq})" $values_path)"
    echo "${patched_values}" > $values_path
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

  jqPath="$(context::_convert_user_path_to_jq_path "${1}")"
  values::jq "$config" -e "${jqPath}"' | (type == "array") and (index("'$2'") != null)' >/dev/null
}

function values::is_true() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  jqPath="$(context::_convert_user_path_to_jq_path "${1}")"
  values::jq "$config" -e "${jqPath} == true" >/dev/null
}

function values::is_false() {
  local config=""
  if [[ "$1" == "--config" ]] ; then
    config=$1
    shift
  fi

  jqPath="$(context::_convert_user_path_to_jq_path "${1}")"
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
    if values::has $config "$var" ; then
      values::get $config "$var"
      return 0
    fi
  done
  return 1
}

function values::_json_patch() {
  set -f
  patch_path=$VALUES_JSON_PATCH_PATH
  if [[ "$1" == "--config" ]] ; then
    shift
    patch_path=$CONFIG_VALUES_JSON_PATCH_PATH
  fi
  jq -nec --arg op "$1" --arg path "$2" --arg value "${3:-""}" \
    '{"op": $op, "path": $path} + if (($value | length) > 0) then {"value": (try ($value | fromjson) catch $value)} else {} end' >> $patch_path
  set +f
}

function values::_normalize_path_for_json_patch() {
  # add a slash to the beginning
  # switch single-quote to double-quote
  # loop — hide dots in keys, i.e. aaa."bb.bb".ccc -> aaa."bb##DOT##bb".cc
  # delete double-quotes
  # switch dots to slashes
  # return original dots from ##DOT##
  sed -E \
    -e 's/^/\//' \
    -e s/\'/\"/g \
    -e ':loop' -e 's/"([^".]+)\.([^"]+)"/"\1##DOT##\2"/g' -e 't loop' \
    -e 's/"//g' \
    -e 's/\./\//g' \
    -e 's/##DOT##/./g' \
    <<< ${1}
}
