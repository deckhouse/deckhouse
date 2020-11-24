#!/bin/bash
function common_hooks::https::copy_custom_certificate::config() {
  cat << EOF
    configVersion: v1
    beforeHelm: 10 # to handle <module>.https.customCertificate.secretName change in cm/deckhouse
    kubernetes:
    - name: secrets
      group: main
      keepFullObjectsInMemory: false
      apiVersion: v1
      kind: Secret
      queue: /modules/$(module::name::kebab_case)/copy_custom_certificate
      namespace:
        nameSelector:
          matchNames: [d8-system]
      labelSelector:
        matchExpressions:
        - key: owner
          operator: NotIn
          values: ["helm"]
      jqFilter: |
        {
          "name": .metadata.name,
          "data": .data
        }
EOF
}

function common_hooks::https::copy_custom_certificate::main() {
  module_name="$( module::name::camel_case )"
  https_mode="$( values::get_first_defined "${module_name}.https.mode" "global.modules.https.mode" )"
  if [[ "$https_mode" == "CustomCertificate" ]] ; then
    if secret_name="$(values::get_first_defined "${module_name}.https.customCertificate.secretName" "global.modules.https.customCertificate.secretName")" ; then
      # shellcheck disable=SC2016
      if secret_data="$(context::jq -er --arg name "$secret_name" '.snapshots.secrets[] | select(.filterResult.name == $name) | .filterResult.data')"; then
        if ! values::has "${module_name}.internal"; then
          values::set "${module_name}.internal" "{}"
        fi
        if ! values::has "${module_name}.internal.customCertificateData"; then
          values::set "${module_name}.internal.customCertificateData" "{}"
        fi
        values::set "${module_name}.internal.customCertificateData" "$secret_data"
      else
        >&2 echo "ERROR: custom certificate secret name is configured, but secret with this name doesn't exist."
        return 1
      fi
    fi
  else
    values::unset "${module_name}.internal.customCertificateData"
  fi
}
