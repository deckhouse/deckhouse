#!/bin/bash

function hook::context_jq() {
  jq "$@" ${BINDING_CONTEXT_PATH}
}
