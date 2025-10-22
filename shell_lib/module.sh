#!/bin/bash -e

# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

function module::name::camel_case() {
  # /deckhouse/modules/301-prometheus-metrics-adapter/hooks/superhook.sh -> prometheusMetricsAdapter
  echo $0 | sed -E 's/^.*\/[0-9]+-([a-zA-Z0-9-]+)\/.+/\1/' | awk -F - '{printf "%s", $1; for(i=2; i<=NF; i++) printf "%s", toupper(substr($i,1,1)) substr($i,2); print"";}'
}

function module::name::kebab_case() {
  # /deckhouse/modules/301-prometheus-metrics-adapter/hooks/superhook.sh -> prometheus-metrics-adapter
  echo $0 | sed -E 's/^.*\/[0-9]+-([a-zA-Z0-9-]+)\/.+/\1/'
}

function module::path() {
  # /deckhouse/modules/301-prometheus-metrics-adapter/hooks/superhook.sh -> /deckhouse/modules/301-prometheus-metrics-adapter
  echo $0 | sed -E 's/^(.*\/[0-9]+-[a-zA-Z0-9-]+)\/.+/\1/'
}

# $1 — target service name
function module::public_domain() {
  TEMPLATE=$(values::get --config --required global.modules.publicDomainTemplate)
  regexp_pattern="^(%s([-a-z0-9]*[a-z0-9])?|[a-z0-9]([-a-z0-9]*)?%s([-a-z0-9]*)?[a-z0-9]|[a-z0-9]([-a-z0-9]*)?%s)(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
  if [[ "$TEMPLATE" =~ ${regexp_pattern} ]]; then
    printf "$TEMPLATE" "$1"
  else
    >&2 echo "ERROR: global.modules.publicDomainTemplate must contain '%s'."
    return 1
  fi
}

function module::ingress_class() {
  module_name=$(module::name::camel_case)
  if values::has ${module_name}.ingressClass ; then
    echo "$(values::get ${module_name}.ingressClass)"
  elif values::has global.modules.ingressClass; then
    echo "$(values::get global.modules.ingressClass)"
  else
    echo "nginx"
  fi
}

module::https_secret_name() {
  prefix_name="$1"
  module_name=$(module::name::camel_case)
  https_mode="$(values::get_first_defined "${module_name}.https.mode" "global.modules.https.mode" )" || true
  case $https_mode in
    "CustomCertificate")
      echo "${prefix_name}-customcertificate"
      ;;
    "CertManager")
      echo "${prefix_name}"
      ;;
    "OnlyInURI")
      echo ""
      ;;
    *)
      >&2 echo "ERROR: https.mode must be in [CertManager, CustomCertificate, OnlyInURI]"
      return 1
      ;;
  esac
}
