#!/bin/bash

function context::global::jq() {
  jq "$@" ${BINDING_CONTEXT_PATH}
}

# $1 — jq filter for current context
function context::jq() {
  context::global::jq '.['"${BINDING_CONTEXT_CURRENT_INDEX}"']' | jq "$@"
}

function context::get() {
  local required=no

  while true ; do
    case ${1:-} in
      --required)
        required=yes
        shift
        ;;
      *)
        break
        ;;
    esac
  done

  if [[ "$required" == "yes" ]] && ! context::has "${1:-}"; then
      >&2 echo "Error: Value $1 required, but doesn't exist"
      return 1
  fi

  jqPath="$(values::_convert_user_path_to_jq_path "${1:-}")"
  context::jq -r "${jqPath}"
}

function context::has() {
  local path=$(values::_dirname "${1:-}")
  local key=$(values::_basename "${1:-}")

  quotes='"'
  if [[ "$key" =~ ^[0-9]+$ ]]; then
    quotes=''
  fi

  jqPath="$(values::_convert_user_path_to_jq_path "${path}")"
  context::jq -e "${jqPath} | has(${quotes}${key}${quotes})" >/dev/null
}

function context::is_true() {
  jqPath="$(values::_convert_user_path_to_jq_path "${1:-}")"
  context::jq -e "${jqPath} == true" >/dev/null
}

function context::is_false() {
  jqPath="$(values::_convert_user_path_to_jq_path "${1:-}")"
  context::jq -e "${jqPath} == false" >/dev/null
}
