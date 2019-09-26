#!/bin/bash

function common_hooks::https::copy_custom_certificate::config() {
  echo '
{
  "afterHelm": 10,
  "onKubernetesEvent": [
    {
      "kind": "secret",
      "event": [
        "update"
      ],
      "namespaceSelector": {
        "matchNames": [
          "antiopa"
        ]
      },
      "jqFilter": "[.data.\"tls.crt\", .data.\"tls.key\"]"
    }
  ]
}'
}

# $1 — имя namespace, куда надо скопировать секрет
function common_hooks::https::copy_custom_certificate::main() {
  module_name=$(module::name)
  secret_name=$(values::get_first_defined ${module_name}.https.customCertificate.secretName global.modules.https.customCertificate.secretName)

  if [ "${secret_name}" != "false" ] && [ ! -z "${secret_name}" ] ; then
    if kubectl -n antiopa get secret ${secret_name} > /dev/null 2>&1 ; then
      kubectl -n antiopa get secret ${secret_name} -o json | \
        jq -r ".metadata.namespace=\"$1\" | .metadata.name=\"$2\" |
          .metadata |= with_entries(select([.key] | inside([\"name\", \"namespace\", \"labels\"])))" \
        | jq 'del(.metadata.labels."antiopa-secret-copier")' \
        | kubectl::replace_or_create
    else
      >&2 echo "You use the customCertificate.secretName, but there is no ${secret_name} secret in antiopa namespace"
      exit 1
    fi
  fi
}

