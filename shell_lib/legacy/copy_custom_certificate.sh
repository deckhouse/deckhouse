#!/bin/bash

function legacy::common_hooks::https::copy_custom_certificate::config() {
  cat << EOF
    configVersion: v1
    afterHelm: 10
    kubernetes:
    - name: secrets
      apiVersion: v1
      kind: Secret
      watchEvent: [Modified]
      queue: /modules/$(module::name::kebab_case)/copy_custom_certificate
      namespace:
        nameSelector:
          matchNames: [d8-system]
      jqFilter: '[.data."tls.crt", .data."tls.key"]'
    $(
    if [ "$#" -gt "0" ]; then
      namespace="$1"
      echo \
   "- name: namespaces
      queue: /modules/$(module::name::kebab_case)/copy_custom_certificate
      group: main
      apiVersion: v1
      kind: Namespace
      watchEvent: [Added]
      nameSelector:
        matchNames: [$namespace]
      jqFilter: '.metadata.name'"
    fi
    )
EOF
}

# $1 — имя namespace, куда надо скопировать секрет
function legacy::common_hooks::https::copy_custom_certificate::main() {
  module_name=$(module::name::camel_case)
  https_mode=$(values::get_first_defined ${module_name}.https.mode global.modules.https.mode)

  if [[ "$https_mode" == "CustomCertificate" ]] ; then
    secret_name=$(values::get_first_defined ${module_name}.https.customCertificate.secretName global.modules.https.customCertificate.secretName)
    if [ "${secret_name}" != "false" ] && [ ! -z "${secret_name}" ] ; then
      if kubectl -n d8-system get secret ${secret_name} >/dev/null 2>&1 ; then
        namespace="d8-system"
      elif kubectl -n antiopa get secret ${secret_name} >/dev/null 2>&1 ; then
        namespace="antiopa"
      else
        >&2 echo "You use the customCertificate.secretName, but there is no ${secret_name} secret in d8-system namespace"
        exit 1
      fi
      if kubectl get ns "$1" 2>/dev/null >/dev/null; then
        kubectl -n ${namespace} get secret ${secret_name} -o json | \
          jq -r ".metadata.namespace=\"$1\" | .metadata.name=\"$2\" |
            .metadata |= with_entries(select([.key] | inside([\"name\", \"namespace\", \"labels\"])))" \
          | jq 'del(.metadata.labels."antiopa-secret-copier")' \
          | jq 'del(.metadata.labels."secret-copier.deckhouse.io/enabled")' \
          | kubernetes::replace_or_create_json
      fi
    fi
  fi
}
