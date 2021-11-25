#!/bin/bash

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

function legacy::common_hooks::https::delete_not_matching_certificate_secret::config() {
  cat << EOF
    configVersion: v1
    beforeHelm: 10
EOF
}

# $1 — имя namespace, где надо удалить секрет
function legacy::common_hooks::https::delete_not_matching_certificate_secret::main() {
  namespace=$1
  module_name=$(module::name::camel_case)
  https_mode=$(values::get_first_defined ${module_name}.https.mode global.modules.https.mode)
  if kubectl get namespace ${namespace} > /dev/null 2>&1 ; then
    if [ "$https_mode" == "CertManager" ]; then
      certificate_issuer_name=$(values::get_first_defined ${module_name}.https.certManager.clusterIssuerName global.modules.https.certManager.clusterIssuerName)
      if [ ! -z "${certificate_issuer_name}" ] ; then
        if kubectl -n ${namespace} get secret ingress-tls > /dev/null 2>&1 ; then
          secret_issuer_name=$(kubectl -n ${namespace} get secret ingress-tls -o json | jq -r '.metadata.annotations."cert-manager.io/issuer-name" // .metadata.annotations."certmanager.k8s.io/issuer-name"')
          if [ "${secret_issuer_name}" != "${certificate_issuer_name}" ] ; then
            kubectl -n ${namespace} delete secret ingress-tls > /dev/null 2>&1
          fi
        fi
      fi
    fi
  fi
}
