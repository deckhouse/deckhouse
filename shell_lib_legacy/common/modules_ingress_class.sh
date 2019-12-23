#!/bin/bash -e

function common::module_ingress_class() {
  module_name=$(module::name)
  if values::has ${module_name}.ingressClass ; then
    echo "$(values::get ${module_name}.ingressClass)"
  elif values::has global.modules.ingressClass; then
    echo "$(values::get global.modules.ingressClass)"
  else
    echo "nginx"
  fi
}
