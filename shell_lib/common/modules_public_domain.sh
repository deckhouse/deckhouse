#!/bin/bash -e

# $1 — имя сервиса, для которого рендерится домен
function common::module_public_domain() {
  TEMPLATE=$(values::get --config --required global.modules.publicDomainTemplate)
  echo "$TEMPLATE" | grep -q '%s' || { echo "Error! global.modules.publicDomainTemplate must contain '%s'."; exit 1;}
  printf "$TEMPLATE" "$1"
}
