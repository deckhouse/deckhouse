#!/bin/bash

# overriding of shell-operator/frameworks/shell/hook.sh#hook::run
function hook::run() {
  if [[ "${1:-}" == "--config" ]] ; then
    __config__
    exit 0
  fi

  CONTEXT_LENGTH=$(context::global::jq -r 'length')
  for i in `seq 0 $((CONTEXT_LENGTH - 1))`; do
    export BINDING_CONTEXT_CURRENT_INDEX="${i}"
    export BINDING_CONTEXT_CURRENT_BINDING=$(context::jq -r '.binding // "unknown"')

    case "${BINDING_CONTEXT_CURRENT_BINDING}" in
    "beforeAll")
      HANDLERS="__on_before_all"
    ;;
    "afterAll")
      HANDLERS="__on_after_all"
    ;;
    "beforeHelm")
      HANDLERS="__on_before_helm"
    ;;
    "afterHelm")
      HANDLERS="__on_after_helm"
    ;;
    *)
      HANDLERS=$(hook::_determine_kubernetes_and_scheduler_handlers)
    esac
    HANDLERS="${HANDLERS} __main__"

    export D8_KUBERNETES_PATCH_SET_FILE=$(kubernetes::_init_patch_set)
    hook::_run_first_available_handler "${HANDLERS}"
    kubernetes::_apply_patch_set
  done
}
