#!/bin/bash

function hook::run() {
  if [[ "${1:-}" == "--config" ]] ; then
    __config__
    exit 0
  fi

  CONTEXT_LENGTH=$(context::global::jq -r 'length')
  for i in `seq 0 $((CONTEXT_LENGTH - 1))`; do
    export BINDING_CONTEXT_CURRENT_INDEX="${i}"
    export BINDING_CONTEXT_CURRENT_BINDING=$(context::jq -r '.binding // "unknown"')

    HANDLER="__undefined"
    HANDLER_SPARE="__undefined"
    HANDLER_SPARE_SPARE="__undefined"
    case "${BINDING_CONTEXT_CURRENT_BINDING}" in
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
      # if current context has .type field
      if BINDING_CONTEXT_CURRENT_TYPE=$(context::jq -er '.type'); then
        HANDLER_SPARE_SPARE="__on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}"
        HANDLER="__on_kubernetes"
        case "${BINDING_CONTEXT_CURRENT_TYPE}" in
        "Synchronization")
          HANDLER="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::synchronization"
        ;;
        "Event")
          case "$(context::jq -r '.watchEvent')" in
          "Added")
            HANDLER_SPARE="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::added_or_modified"
            HANDLER="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::added"
          ;;
          "Modified")
            HANDLER_SPARE="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::added_or_modified"
            HANDLER="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::modified"
          ;;
          "Deleted")
            HANDLER="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::deleted"
          ;;
          esac
        ;;
        esac
      else
        HANDLER="__on_schedule::${BINDING_CONTEXT_CURRENT_BINDING}"
      fi
    ;;
    esac

    export D8_KUBERNETES_PATCH_SET_FILE=$(kubernetes::_init_patch_set)
    if type $HANDLER >/dev/null 2>&1; then
      $HANDLER
    elif type $HANDLER_SPARE >/dev/null 2>&1; then
      $HANDLER_SPARE
    elif type $HANDLER_SPARE_SPARE >/dev/null 2>&1; then
      $HANDLER_SPARE_SPARE
    elif type __main__ >/dev/null 2>&1; then
      __main__
    else
      >&2 echo -n "ERROR: Can't find handler '${HANDLER}'"
      if [[ "${HANDLER_SPARE}" != "__undefined" ]]; then
        >&2 echo -n " or '${HANDLER_SPARE}'"
      fi
      if [[ "${HANDLER_SPARE_SPARE}" != "__undefined" ]]; then
        >&2 echo -n " or '${HANDLER_SPARE_SPARE}'"
      fi
      >&2 echo " or '__main__'"
      exit 1
    fi
    kubernetes::_apply_patch_set
  done
}

function hook::generate::store_handlers() {
  bindingContextBinding="$1"
  storePath="$2"
  key=$3
  cat << EOF
    function __on_kubernetes::${bindingContextBinding}::synchronization() {
      values::set "${storePath}" "\$(context::jq -r '[.objects[] | select(.filterResult != "" and .filterResult != null) | .filterResult]')"
    }

    function __on_kubernetes::${bindingContextBinding}::added_or_modified() {
      filterResult="\$(context::get filterResult)"
      if ! tools::is_empty "\${filterResult}"; then
        values::store::replace_row_by_key "${storePath}" "${key}" "\${filterResult}"
      fi
    }

    function __on_kubernetes::${bindingContextBinding}::deleted() {
      filterResult="\$(context::get filterResult)"
      if ! tools::is_empty "\${filterResult}"; then
        values::store::unset_row_by_key "${storePath}" "${key}" "\${filterResult}"
      fi
    }
EOF
}
