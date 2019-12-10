#!/bin/bash

function hook::run() {
  if [[ "${1:-}" == "--config" ]] ; then
    __config__
  else
    __main__
  fi
}

function hook::run_ng() {
  if [[ "${1:-}" == "--config" ]] ; then
    __config__
    exit 0
  fi

  CONTEXT_LENGTH=$(hook::context_jq -r '. | length')
  for i in `seq 0 $((CONTEXT_LENGTH - 1))`; do
    export BINDING_CONTEXT_BINDING=$(hook::context_jq -r '.['$i'].binding // "unknown"')
    ARG1=""
    ARG2=""

    HANDLER="undefined"
    HANDLER_SPARE="undefined"
    case "${BINDING_CONTEXT_BINDING}" in
    "onStartup")
      HANDLER="__on_startup"
    ;;
    "beforeAll")
      HANDLER="__on_before_all"
    ;;
    "afterAll")
      HANDLER="__on_after_all"
    ;;
    "beforeHelm")
      HANDLER="__on_before_helm"
    ;;
    "afterHelm")
      HANDLER="__on_after_helm"
    ;;
    "afterDeleteHelm")
      HANDLER="__on_after_delete_helm"
    ;;
    *)
      if hook::context_jq -e '.['$i'] | has("type")'; then
        HANDLER="__on_kubernetes"
        export BINDING_CONTEXT_TYPE="$(hook::context_jq -r '.['$i'].type')"
        case "${BINDING_CONTEXT_TYPE}" in
        "Synchronization")
          HANDLER="${HANDLER}::synchronization::${BINDING_CONTEXT_BINDING}"
          ARG1="$(hook::context_jq -cr '[(.['$i'].objects // [])[] | select(has("filterResult")) | if .filterResult == "" then "\"\"" else .filterResult end | fromjson]')"
          ARG2="$(hook::context_jq -c  '[(.['$i'].objects // [])[] | .object]')"
        ;;
        "Event")
          export BINDING_CONTEXT_WATCH_EVENT="$(hook::context_jq -r '.['$i'].watchEvent')"
          ARG1="$(hook::context_jq -cr '.['$i'].filterResult // "\"\"" | fromjson')"
          ARG2="$(hook::context_jq -c  '.['$i'].object')"
          case "${BINDING_CONTEXT_WATCH_EVENT}" in
          "Added")
            HANDLER_SPARE="${HANDLER}::added_or_modified::${BINDING_CONTEXT_BINDING}"
            HANDLER="${HANDLER}::added::${BINDING_CONTEXT_BINDING}"
          ;;
          "Modified")
            HANDLER_SPARE="${HANDLER}::added_or_modified::${BINDING_CONTEXT_BINDING}"
            HANDLER="${HANDLER}::modified::${BINDING_CONTEXT_BINDING}"
          ;;
          "Deleted")
            HANDLER="${HANDLER}::deleted::${BINDING_CONTEXT_BINDING}"
          ;;
          esac
        ;;
        esac
      else
        HANDLER="__on_schedule::${BINDING_CONTEXT_BINDING}"
      fi
    ;;
    esac

    export D8_KUBERNETES_PATCH_SET_FILE=$(kubernetes::init_patch_set)
    if type $HANDLER >/dev/null 2>&1; then
      $HANDLER "$ARG1" "$ARG2"
    elif type $HANDLER_SPARE >/dev/null 2>&1; then
      $HANDLER_SPARE "$ARG1" "$ARG2"
    elif type __main__ >/dev/null 2>&1; then
      __main__
    else
      >&2 echo -n "ERROR: Can't find handler '${HANDLER}'"
      if [ -n "${HANDLER_SPARE}" ]; then
        >&2 echo -n " or '${HANDLER_SPARE}'"
      fi
      >&2 echo " or '__main__'"
      exit 1
    fi
    kubernetes::apply_patch_set
  done
}

function hook::generate::store_handlers() {
  bindingContextBinding="$1"
  storePath="$2"
  key=$3
  cat << EOF
    function __on_kubernetes::synchronization::${bindingContextBinding}() {
      values::set "${storePath}" "\${1}"
    }

    function __on_kubernetes::added::${bindingContextBinding}() {
      values::store::replace_row_by_key "${storePath}" "${key}" "\${1}"
    }

    function __on_kubernetes::modified::${bindingContextBinding}() {
      values::store::replace_row_by_key "${storePath}" "${key}" "\${1}"
    }

    function __on_kubernetes::deleted::${bindingContextBinding}() {
      values::store::unset_row_by_key "${storePath}" "${key}" "\${1}"
    }
EOF
}
