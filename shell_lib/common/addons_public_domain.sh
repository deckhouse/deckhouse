#!/bin/bash -e

# $1 — имя сервиса, для которого рендерится домен
function common::addon_public_domain() {
  TEMPLATE=$(values::get --config --required global.addonsPublicDomainTemplate)
  echo "$TEMPLATE" | grep -q '%s' || { echo "Error! global.addonsPublicDomainTemplate must contain '%s'."; exit 1;}
  printf "$TEMPLATE" "$1"
}
