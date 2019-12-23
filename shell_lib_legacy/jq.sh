#!/bin/bash

function jq() {
  command jq -L "/deckhouse/jq_lib" "$@"
}
