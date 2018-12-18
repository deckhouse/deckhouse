#!/bin/bash

function jq() {
  command jq -L "/antiopa/jq_lib" "$@"
}
